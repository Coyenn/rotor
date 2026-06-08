package main

import (
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"rotor/internal/compile"
)

const pollInterval = 250 * time.Millisecond

// fileStamp captures enough of a file's state to detect edits via polling.
type fileStamp struct {
	exists  bool
	modTime time.Time
	size    int64
}

// runWatch runs an initial check, then polls the watched file set (the
// parsed file list plus tsconfig.json) and re-runs the full check whenever
// anything changes. Exits only via Ctrl+C.
func runWatch(dir string, out io.Writer) int {
	u := newUI(out)
	res := runCheck(dir, out)
	stamps := snapshotFiles(res.watchFiles)
	u.watchBanner(len(res.watchFiles))

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for range ticker.C {
		changed, file := detectFileChange(res.watchFiles, stamps)
		if !changed {
			continue
		}
		u.watchChange(file)
		res = runCheck(dir, out)
		stamps = snapshotFiles(res.watchFiles)
		u.watchBanner(len(res.watchFiles))
	}
	return 0
}

func timestamp() string {
	return time.Now().Format("15:04:05")
}

func snapshotFiles(files []string) map[string]fileStamp {
	stamps := make(map[string]fileStamp, len(files))
	for _, f := range files {
		stamps[f] = stat(f)
	}
	return stamps
}

func detectFileChange(files []string, stamps map[string]fileStamp) (bool, string) {
	for _, f := range files {
		if stat(f) != stamps[f] {
			return true, f
		}
	}
	return false, ""
}

func runBuildWatch(dir, tsConfigPath string, opts projectOptions) int {
	u := newUI(os.Stdout)
	result, diags, elapsed, err := runBuildOnce(dir, tsConfigPath, opts)
	reportBuildPass(u, result, diags, elapsed, err)

	outputDir := guessedOutputDir(dir, result)
	stamps := snapshotProjectTree(dir, outputDir)

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for range ticker.C {
		next := snapshotProjectTree(dir, outputDir)
		changed, path := detectProjectTreeChange(stamps, next)
		if !changed {
			continue
		}

		u.watchChange(path)

		result, diags, elapsed, err = runBuildOnce(dir, tsConfigPath, opts)
		reportBuildPass(u, result, diags, elapsed, err)

		outputDir = guessedOutputDir(dir, result)
		stamps = snapshotProjectTree(dir, outputDir)
	}
	return 0
}

func reportBuildPass(u *ui, result *compile.BuildResult, diags []string, elapsed time.Duration, err error) {
	if err != nil {
		newUI(os.Stderr).buildFailure(err.Error(), diags)
	} else if result != nil {
		u.buildSuccess(len(result.Outputs), len(result.EmittedFiles), len(result.Outputs)-len(result.EmittedFiles), elapsed)
	}
	u.watchIdle()
}

func snapshotProjectTree(root, outputDir string) map[string]fileStamp {
	stamps := make(map[string]fileStamp)
	walkSnapshotTree(root, root, outputDir, stamps)
	return stamps
}

func walkSnapshotTree(root, dir, outputDir string, stamps map[string]fileStamp) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		if shouldSkipWatchPath(path, outputDir) {
			continue
		}
		if entry.IsDir() {
			walkSnapshotTree(root, path, outputDir, stamps)
			continue
		}
		stamps[path] = stat(path)
	}
}

func shouldSkipWatchPath(path, outputDir string) bool {
	if filepath.Base(path) == ".git" {
		return true
	}
	if outputDir == "" {
		return false
	}
	rel, err := filepath.Rel(outputDir, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func detectProjectTreeChange(before, after map[string]fileStamp) (bool, string) {
	keys := make([]string, 0, len(before)+len(after))
	seen := make(map[string]struct{}, len(before)+len(after))
	for path := range before {
		keys = append(keys, path)
		seen[path] = struct{}{}
	}
	for path := range after {
		if _, ok := seen[path]; ok {
			continue
		}
		keys = append(keys, path)
	}
	sort.Strings(keys)
	for _, path := range keys {
		if before[path] != after[path] {
			return true, path
		}
	}
	return false, ""
}

func guessedOutputDir(projectDir string, result *compile.BuildResult) string {
	if result != nil && result.OutputDir != "" {
		return result.OutputDir
	}
	if result == nil || len(result.Outputs) == 0 {
		return filepath.Join(projectDir, "out")
	}
	relPaths := make([]string, 0, len(result.Outputs))
	for rel := range result.Outputs {
		relPaths = append(relPaths, rel)
	}
	sort.Strings(relPaths)
	first := filepath.Clean(filepath.FromSlash(relPaths[0]))
	parts := strings.Split(first, string(filepath.Separator))
	if len(parts) == 0 {
		return filepath.Join(projectDir, "out")
	}
	return filepath.Join(projectDir, parts[0])
}

func stat(path string) fileStamp {
	info, err := os.Stat(path)
	if err != nil {
		return fileStamp{}
	}
	return fileStamp{exists: true, modTime: info.ModTime(), size: info.Size()}
}
