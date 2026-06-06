package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
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
	stamps := snapshot(res.watchFiles)
	printWatching(out, len(res.watchFiles))

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for range ticker.C {
		changed, file := detectChange(res.watchFiles, stamps)
		if !changed {
			continue
		}
		fmt.Fprintf(out, "\n%s\n", strings.Repeat("-", 72))
		fmt.Fprintf(out, "[%s] change detected (%s) — rechecking...\n\n", timestamp(), file)
		res = runCheck(dir, out)
		stamps = snapshot(res.watchFiles)
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

func snapshot(files []string) map[string]fileStamp {
	stamps := make(map[string]fileStamp, len(files))
	for _, f := range files {
		stamps[f] = stat(f)
	}
	return stamps
}

func detectChange(files []string, stamps map[string]fileStamp) (bool, string) {
	for _, f := range files {
		if stat(f) != stamps[f] {
			return true, f
		}
	}
	return false, ""
}

func stat(path string) fileStamp {
	info, err := os.Stat(path)
	if err != nil {
		return fileStamp{}
	}
	return fileStamp{exists: true, modTime: info.ModTime(), size: info.Size()}
}
