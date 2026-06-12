package config

import (
	"os"
	"path/filepath"
	"testing"
)

// RefreshTypeDeclarations is the auto-refresh hook used by `rotor asset` /
// `rotor deploy` after a successful config load: missing → written, stale →
// rewritten, current → untouched (content-compare; no write).
func TestRefreshTypeDeclarations(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, TypeDeclarationsFileName)

	// missing → written
	wrote, err := RefreshTypeDeclarations(dir)
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
	if string(data) != TypeDeclarations {
		t.Error("written content differs from TypeDeclarations")
	}

	// current → untouched
	wrote, err = RefreshTypeDeclarations(dir)
	if err != nil {
		t.Fatal(err)
	}
	if wrote {
		t.Error("up-to-date file: wrote = true, want false")
	}

	// stale → rewritten
	if err := os.WriteFile(path, []byte("// old declarations\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	wrote, err = RefreshTypeDeclarations(dir)
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
	if string(data) != TypeDeclarations {
		t.Error("stale file not refreshed to current TypeDeclarations")
	}

	// no leftover temp files from the atomic write
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("directory has %d entries, want only %s", len(entries), TypeDeclarationsFileName)
	}
}
