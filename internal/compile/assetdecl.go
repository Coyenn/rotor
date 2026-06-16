package compile

import (
	"strings"

	"rotor/tsgo/tspath"
	"rotor/tsgo/vfs"
	"rotor/tsgo/vfs/wrapvfs"
)

// Synthetic ambient declaration for the rotor $asset compile-time asset macro
// (a SUPERSET extension; see internal/transformer/assetmacro.go). Mirrors the
// $env declaration mechanism (envdecl.go) exactly: rotor controls program
// creation, so instead of asking users to install types, an in-memory
// declaration file is added to every program's root files, and the type
// checker then accepts `$asset("path/to/file.png")` without any project
// changes.
//
// Parity safety: identical to envdecl.go's rationale — the file is a
// declaration file (excluded from compiled sources and CommonSourceDirectory),
// appended AFTER the config-derived root files, global under
// moduleDetection:"force", and code that never mentions `$asset` resolves no
// differently. `$asset` did not compile under rbxtsc (undeclared identifier),
// so inlining it is a strict superset.
const assetDeclFileName = "__rotor_asset.d.ts"

// assetDeclBody is the one true `$asset` ambient declaration. The synthetic
// in-memory file (assetDeclText) and the generated on-disk editor companion
// (AssetDeclFileText) must carry EXACTLY this declaration so the two are
// interchangeable — only their leading comments differ.
const assetDeclBody = `declare function $asset(path: string): string;
`

const assetDeclText = `// rotor compiler extension (synthetic, in-memory): the $asset compile-time
// asset macro. This file is injected by the rotor compiler and does not exist
// on disk.
` + assetDeclBody

// AssetDeclFileName is the legacy per-macro editor companion for $asset,
// superseded by the consolidated rotor.d.ts (rotortypes.go). The name is still
// recognised so a project that carries it keeps type-checking $asset (the
// coexistence guard suppresses the synthetic declaration) and
// `rotor clean --types` removes it.
const AssetDeclFileName = "rotor-asset.d.ts"

// projectDeclaresAssetOnDisk reports whether the parsed program's root files
// already include an on-disk rotor-asset.d.ts that declares $asset. When they
// do, the synthetic in-memory declaration must NOT also be appended — two
// global `declare function $asset` declarations would be a duplicate-identifier
// error. The content check is tolerant (any `declare function $asset` or
// `declare const $asset`).
func projectDeclaresAssetOnDisk(fs vfs.FS, fileNames []string) bool {
	for _, name := range fileNames {
		base := name
		if i := strings.LastIndexByte(name, '/'); i >= 0 {
			base = name[i+1:]
		}
		if !strings.EqualFold(base, AssetDeclFileName) && !strings.EqualFold(base, RotorTypesFileName) {
			continue
		}
		if text, ok := fs.ReadFile(name); ok &&
			(strings.Contains(text, "declare function $asset") || strings.Contains(text, "declare const $asset")) {
			return true
		}
	}
	return false
}

// assetDeclPath places the synthetic declaration next to the tsconfig (slash
// separated, like all vfs paths).
func assetDeclPath(configPath string) string {
	return tspath.GetDirectoryPath(configPath) + "/" + assetDeclFileName
}

// injectAssetDeclFS wraps fs so the synthetic declaration file exists at
// declPath. Same two-interception shape as injectEnvDeclFS.
func injectAssetDeclFS(inner vfs.FS, declPath string) vfs.FS {
	matches := func(path string) bool {
		if inner.UseCaseSensitiveFileNames() {
			return path == declPath
		}
		return strings.EqualFold(path, declPath)
	}
	return wrapvfs.Wrap(inner, wrapvfs.Replacements{
		FileExists: func(path string) bool {
			return matches(path) || inner.FileExists(path)
		},
		ReadFile: func(path string) (string, bool) {
			if matches(path) {
				return assetDeclText, true
			}
			return inner.ReadFile(path)
		},
	})
}
