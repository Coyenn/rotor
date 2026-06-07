package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSnapshotProjectTreeExcludesOutputDirAndDetectsAddedFiles(t *testing.T) {
	root := t.TempDir()
	srcFile := filepath.Join(root, "src", "main.ts")
	outFile := filepath.Join(root, "out", "main.luau")
	if err := os.MkdirAll(filepath.Dir(srcFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(outFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(srcFile, []byte("export {};\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outFile, []byte("-- generated\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	before := snapshotProjectTree(root, filepath.Join(root, "out"))
	if _, ok := before[srcFile]; !ok {
		t.Fatalf("snapshot missing source file: %v", before)
	}
	if _, ok := before[outFile]; ok {
		t.Fatalf("snapshot included output file: %v", before)
	}

	added := filepath.Join(root, "src", "extra.json")
	if err := os.WriteFile(added, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	after := snapshotProjectTree(root, filepath.Join(root, "out"))
	changed, path := detectProjectTreeChange(before, after)
	if !changed || path != added {
		t.Fatalf("detectProjectTreeChange = (%v, %q), want (%v, %q)", changed, path, true, added)
	}
}

func TestDetectProjectTreeChangeReportsRemovedFiles(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "src", "gone.ts")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("export {};\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	before := snapshotProjectTree(root, filepath.Join(root, "out"))
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	after := snapshotProjectTree(root, filepath.Join(root, "out"))
	changed, got := detectProjectTreeChange(before, after)
	if !changed || got != path {
		t.Fatalf("detectProjectTreeChange = (%v, %q), want (%v, %q)", changed, got, true, path)
	}
}
