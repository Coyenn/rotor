package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"rotor/internal/compile"
	"rotor/internal/logservice"
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
	res := runCheck(dir, out)
	stamps := snapshotFiles(res.watchFiles)
	printWatching(out, len(res.watchFiles))

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for range ticker.C {
		changed, file := detectFileChange(res.watchFiles, stamps)
		if !changed {
			continue
		}
		fmt.Fprintf(out, "\n%s\n", strings.Repeat("-", 72))
		fmt.Fprintf(out, "[%s] change detected (%s) — rechecking...\n\n", timestamp(), file)
		res = runCheck(dir, out)
		stamps = snapshotFiles(res.watchFiles)
		printWatching(out, len(res.watchFiles))
	}
	return 0
}

func printWatching(out io.Writer, n int) {
	fmt.Fprintf(out, "\n[%s] watching %d files for changes (Ctrl+C to exit)\n", timestamp(), n)
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
	result, diags, elapsed, err := runBuildOnce(dir, tsConfigPath, opts)
	reportBuildPass(result, diags, elapsed, err)

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

		fmt.Printf("\n%s\n", strings.Repeat("-", 72))
		fmt.Printf("[%s] File change detected. Starting incremental compilation... (%s)\n\n", timestamp(), path)

		result, diags, elapsed, err = runBuildOnce(dir, tsConfigPath, opts)
		reportBuildPass(result, diags, elapsed, err)

		outputDir = guessedOutputDir(dir, result)
		stamps = snapshotProjectTree(dir, outputDir)
	}
	return 0
}

func reportBuildPass(result *compile.BuildResult, diags []string, elapsed time.Duration, err error) {
	errorCount := len(diags)
	for _, d := range diags {
		fmt.Fprintln(os.Stderr, d)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "rotor build: %v\n", err)
	}
	if result != nil {
		logservice.WriteLine(fmt.Sprintf("compiled %d files (%d written) in %d ms",
			len(result.Outputs), len(result.EmittedFiles), elapsed.Milliseconds()))
	}
	fmt.Printf("[%s] Found %d error%s. Watching for file changes.\n", timestamp(), errorCount, pluralS(errorCount))
}

func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
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
