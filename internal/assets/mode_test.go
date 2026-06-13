package assets

import (
	"os"
	"path/filepath"
	"testing"
)

func outPaths(luau, types string) struct {
	Luau  string
	Types string
} {
	return struct {
		Luau  string
		Types string
	}{Luau: luau, Types: types}
}

// Macro mode: EmitForMode writes the rotor-asset.d.ts companion and does NOT
// write assets.luau, even when output paths are configured.
func TestEmitForModeMacroWritesCompanionNotLuau(t *testing.T) {
	dir := t.TempDir()
	lock := NewLockfile()
	lock.Assets["assets/logo.png"] = LockEntry{Hash: "sha256:x", AssetID: 123}

	companion := MacroCompanion{FileName: "rotor-asset.d.ts", Text: "// generated\ndeclare function $asset(path: string): string;\n"}
	written, err := EmitForMode(dir, ModeMacro, outPaths("src/shared/assets.luau", "src/shared/assets.d.ts"), companion, lock)
	if err != nil {
		t.Fatal(err)
	}
	if len(written) != 1 || written[0] != "rotor-asset.d.ts" {
		t.Fatalf("written = %v, want [rotor-asset.d.ts]", written)
	}
	data, err := os.ReadFile(filepath.Join(dir, "rotor-asset.d.ts"))
	if err != nil {
		t.Fatalf("companion not written: %v", err)
	}
	if string(data) != companion.Text {
		t.Errorf("companion content = %q, want %q", string(data), companion.Text)
	}
	// assets.luau must NOT exist in macro mode.
	if _, err := os.Stat(filepath.Join(dir, "src", "shared", "assets.luau")); err == nil {
		t.Error("macro mode must not write assets.luau")
	}

	// Idempotent: a second call with the same companion writes nothing.
	written, err = EmitForMode(dir, ModeMacro, outPaths("", ""), companion, lock)
	if err != nil {
		t.Fatal(err)
	}
	if len(written) != 0 {
		t.Errorf("re-emit wrote %v, want nothing (companion current)", written)
	}
}

// Module mode (default): EmitForMode regenerates assets.luau + assets.d.ts and
// writes no rotor-asset.d.ts — unchanged 1.x behaviour.
func TestEmitForModeModuleWritesLuauAndTypes(t *testing.T) {
	dir := t.TempDir()
	lock := NewLockfile()
	lock.Assets["assets/logo.png"] = LockEntry{Hash: "sha256:x", AssetID: 123}

	companion := MacroCompanion{FileName: "rotor-asset.d.ts", Text: "// generated\n"}
	written, err := EmitForMode(dir, ModeModule, outPaths("assets.luau", "assets.d.ts"), companion, lock)
	if err != nil {
		t.Fatal(err)
	}
	if len(written) != 2 {
		t.Fatalf("written = %v, want assets.luau + assets.d.ts", written)
	}
	if _, err := os.Stat(filepath.Join(dir, "assets.luau")); err != nil {
		t.Errorf("module mode must write assets.luau: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "assets.d.ts")); err != nil {
		t.Errorf("module mode must write assets.d.ts: %v", err)
	}
	// No rotor-asset.d.ts companion in module mode.
	if _, err := os.Stat(filepath.Join(dir, "rotor-asset.d.ts")); err == nil {
		t.Error("module mode must not write rotor-asset.d.ts")
	}
}

// ParseMode defaults the empty string to module and passes other values
// through (config.Validate rejects bad values upstream).
func TestParseMode(t *testing.T) {
	cases := map[string]Mode{"": ModeModule, "module": ModeModule, "macro": ModeMacro}
	for in, want := range cases {
		if got := ParseMode(in); got != want {
			t.Errorf("ParseMode(%q) = %q, want %q", in, got, want)
		}
	}
}
