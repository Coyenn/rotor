package assets

import (
	"fmt"
	"os"
	"path/filepath"
)

// Mode is the asset delivery mode from [assets].mode in rotor.toml.
type Mode string

const (
	// ModeModule (default) generates the assets.luau accessor module plus its
	// assets.d.ts declaration from the lockfile — the 1.x behaviour.
	ModeModule Mode = "module"
	// ModeMacro maintains the lockfile and writes the consolidated rotor.d.ts
	// editor companion (no assets.luau); the $asset transformer is the
	// consumption path, with build-time auto-upload filling any gaps.
	ModeMacro Mode = "macro"
)

// ParseMode maps the raw config string to a Mode, defaulting to ModeModule for
// the empty string. Any other value is returned verbatim (config.Validate
// already rejects it before sync runs).
func ParseMode(raw string) Mode {
	switch raw {
	case "", string(ModeModule):
		return ModeModule
	case string(ModeMacro):
		return ModeMacro
	default:
		return Mode(raw)
	}
}

// MacroCompanion names the on-disk editor companion written in macro mode and
// its content. The single source of truth for the declaration lives in
// internal/compile (compile.RotorTypesFileName / RotorTypesFileText); the caller
// passes them through so internal/assets need not depend on internal/compile.
type MacroCompanion struct {
	FileName string // e.g. "rotor.d.ts"
	Text     string // the full file content (with generated-file header)
}

// EmitForMode performs the mode-specific output step of `rotor asset sync`,
// AFTER the lockfile is up to date:
//
//   - ModeModule: regenerate assets.luau + assets.d.ts from the lockfile at the
//     configured output paths (unchanged 1.x behaviour). Empty paths are
//     skipped.
//   - ModeMacro: write the rotor.d.ts editor companion (atomically, only when
//     missing or stale) and write NO assets.luau, even if output paths are
//     configured.
//
// Both modes share the same lockfile/scan/hash/upload pipeline; only this final
// emit differs. Returns the paths written (project-relative or as given) for
// reporting.
func EmitForMode(projectDir string, mode Mode, out struct {
	Luau  string
	Types string
}, companion MacroCompanion, lock *Lockfile) (written []string, err error) {
	switch mode {
	case ModeMacro:
		if companion.FileName == "" {
			return nil, nil
		}
		path := filepath.Join(projectDir, filepath.FromSlash(companion.FileName))
		if existing, rerr := os.ReadFile(path); rerr == nil && string(existing) == companion.Text {
			return nil, nil // current → untouched
		}
		if werr := writeFileAtomic(path, []byte(companion.Text)); werr != nil {
			return nil, fmt.Errorf("assets: writing %s: %w", companion.FileName, werr)
		}
		return []string{companion.FileName}, nil
	default: // ModeModule
		if out.Luau == "" && out.Types == "" {
			return nil, nil
		}
		if werr := WriteOutputs(projectDir, out.Luau, out.Types, lock); werr != nil {
			return nil, werr
		}
		for _, p := range []string{out.Luau, out.Types} {
			if p != "" {
				written = append(written, p)
			}
		}
		return written, nil
	}
}
