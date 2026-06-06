package compile

import (
	"encoding/json"
	"strings"

	"rotor/tsgo/vfs"
	"rotor/tsgo/vfs/wrapvfs"
)

// SanitizeFS wraps an FS so that any ReadFile of a file named exactly
// "tsconfig.json" returns sanitized config text (see SanitizeTSConfig). The
// wrapper is general: it applies to every tsconfig.json the compiler host
// touches (the project's own config plus anything reached via "extends").
func SanitizeFS(inner vfs.FS) vfs.FS {
	return wrapvfs.Wrap(inner, wrapvfs.Replacements{
		ReadFile: func(path string) (string, bool) {
			contents, ok := inner.ReadFile(path)
			if !ok || !isTSConfigPath(path) {
				return contents, ok
			}
			return SanitizeTSConfig(contents), true
		},
	})
}

// isTSConfigPath reports whether path's basename is exactly "tsconfig.json"
// (vfs paths are slash-separated). A bare suffix match would also intercept
// unrelated files like "my-tsconfig.json".
func isTSConfigPath(path string) bool {
	return path == "tsconfig.json" || strings.HasSuffix(path, "/tsconfig.json")
}

// SanitizeTSConfig rewrites a tsconfig.json that rbxtsc requires into one
// tsgo (TS7) accepts. tsgo's Program.verifyCompilerOptions (tsgo/compiler/
// program.go, "Removed in TS7" section) hard-errors on three options that
// rbxtsc's project validation REQUIRES users to set:
//
//   - "downlevelIteration" (any value)  -> removed
//   - "baseUrl" (any value)             -> removed
//   - "moduleResolution": "Node"/node10 -> replaced with "bundler"
//
// One TS5->TS7 semantic repair on top of the removals: TS7 no longer
// auto-includes every package under typeRoots when "types" is unspecified
// (module.GetAutomaticTypeDirectiveNames returns [] unless the types array
// holds the "*" wildcard — tsgo/module/resolver.go:2075-2081). rbxtsc
// projects depend on auto-inclusion to load @rbxts/types globals (print,
// Array, ...), so when "types" is absent the sanitizer injects
// "types": ["*"], which reproduces TS5's typeRoots walk exactly. An explicit
// "types" array is left untouched (TS5 also disabled auto-inclusion then).
//
// moduleResolution choice, determined empirically against the fixture project
// (testdata/diff/project, module=commonjs, typeRoots=node_modules/@rbxts):
//   - "node16" FAILS: tsgo emits "Option 'module' must be set to 'Node16'
//     when option 'moduleResolution' is set to 'Node16'" (program.go:1140-44)
//     because the fixture's module is commonjs.
//   - removing the key entirely WORKS: CompilerOptions.GetModuleResolutionKind
//     defaults Unknown to bundler when module is commonjs
//     (tsgo/core/compileroptions.go:221-231) — zero diagnostics.
//   - "bundler" WORKS: explicitly exempted for module=commonjs
//     (program.go:1126) — zero diagnostics, and bundler resolution still finds
//     node_modules packages and @rbxts typeRoots.
//
// "bundler" is chosen over key-removal because it pins the same resolution
// kind the default would pick today, guarding against future default changes.
//
// Input may be JSONC (// and /* */ comments, trailing commas — both legal in
// tsconfig.json); output is plain JSON. Key order is not preserved, which is
// fine: tsconfig key order carries no meaning. Malformed input is returned
// untouched so tsoptions reports the parse error itself.
func SanitizeTSConfig(src string) string {
	clean := stripJSONC(src)
	var root map[string]any
	if err := json.Unmarshal([]byte(clean), &root); err != nil {
		return src
	}
	co, ok := root["compilerOptions"].(map[string]any)
	if !ok {
		return clean
	}
	delete(co, "downlevelIteration")
	delete(co, "baseUrl")
	if mr, ok := co["moduleResolution"].(string); ok {
		// Case-insensitive value match: tsconfig enum values are
		// case-insensitive ("Node" parses as node10).
		switch strings.ToLower(mr) {
		case "node", "node10":
			co["moduleResolution"] = "bundler"
		}
	}
	if _, hasTypes := co["types"]; !hasTypes {
		co["types"] = []any{"*"}
	}
	out, err := json.MarshalIndent(root, "", "\t")
	if err != nil {
		return clean
	}
	return string(out)
}

// stripJSONC removes // line comments, /* */ block comments, and trailing
// commas (all outside string literals) so encoding/json can parse tsconfig
// text.
func stripJSONC(src string) string {
	var b strings.Builder
	b.Grow(len(src))
	inString := false
	for i := 0; i < len(src); i++ {
		c := src[i]
		if inString {
			b.WriteByte(c)
			if c == '\\' && i+1 < len(src) {
				i++
				b.WriteByte(src[i])
				continue
			}
			if c == '"' {
				inString = false
			}
			continue
		}
		switch {
		case c == '"':
			inString = true
			b.WriteByte(c)
		case c == '/' && i+1 < len(src) && src[i+1] == '/':
			for i < len(src) && src[i] != '\n' {
				i++
			}
			if i < len(src) {
				b.WriteByte('\n')
			}
		case c == '/' && i+1 < len(src) && src[i+1] == '*':
			i += 2
			for i+1 < len(src) && !(src[i] == '*' && src[i+1] == '/') {
				i++
			}
			i++ // land on '/'; loop increment steps past it
		case c == ',':
			// Trailing comma: drop if the next non-whitespace,
			// non-comment byte closes an object/array. Cheap scan that
			// only skips whitespace — a comma separated from its
			// closer by a comment is rare enough to leave to the JSON
			// parser.
			j := i + 1
			for j < len(src) && (src[j] == ' ' || src[j] == '\t' || src[j] == '\r' || src[j] == '\n') {
				j++
			}
			if j < len(src) && (src[j] == '}' || src[j] == ']') {
				continue
			}
			b.WriteByte(c)
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}
