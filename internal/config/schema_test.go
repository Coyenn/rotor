package config

import (
	"os"
	"path/filepath"
	"testing"
)

// writeConfigFile writes content to dir/name, creating parent dirs.
func writeConfigFile(t *testing.T, dir, name, content string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// RefreshSchema is the auto-refresh hook used by `rotor asset` / `rotor deploy`
// after a successful config load: missing → written, stale → rewritten,
// current → untouched (content-compare; no write).
func TestRefreshSchema(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, SchemaFileName)

	// missing → written
	wrote, err := RefreshSchema(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !wrote {
		t.Error("missing file: wrote = false, want true")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != Schema {
		t.Error("written content differs from Schema")
	}

	// current → untouched
	wrote, err = RefreshSchema(dir)
	if err != nil {
		t.Fatal(err)
	}
	if wrote {
		t.Error("up-to-date file: wrote = true, want false")
	}

	// stale → rewritten
	if err := os.WriteFile(path, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	wrote, err = RefreshSchema(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !wrote {
		t.Error("stale file: wrote = false, want true")
	}
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != Schema {
		t.Error("stale file not refreshed to current Schema")
	}

	// no leftover temp files from the atomic write
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("directory has %d entries, want only %s", len(entries), SchemaFileName)
	}
}

func TestWriteSchema(t *testing.T) {
	dir := t.TempDir()
	if err := WriteSchema(dir); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, SchemaFileName))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != Schema {
		t.Fatal("written file does not match Schema")
	}
}
