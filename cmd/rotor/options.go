package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"rotor/internal/compile"
)

// projectOptions is the Go analog of upstream ProjectOptions
// (Shared/types.ts L4-18): the fully-resolved option set a build runs with.
// `project` is CLI-only (a yargs option default, not part of ProjectOptions).
type projectOptions struct {
	includePath            string
	rojo                   string
	typeName               string // ProjectType choice; "" = infer (upstream undefined)
	watch                  bool
	usePolling             bool
	verbose                bool
	noInclude              bool
	logTruthyChanges       bool
	writeOnlyChanged       bool
	writeTransformedFiles  bool
	optimizedLoops         bool
	allowCommentDirectives bool
	luau                   bool

	// minify is a rotor extension (no rbxtsc analog): pass every emitted
	// .luau/.lua source through the Luau minifier before writing. Set from
	// --minify; never merged from the rbxts tsconfig key.
	minify bool
}

// defaultProjectOptions ports DEFAULT_PROJECT_OPTIONS (Shared/constants.ts
// L41-55). Only optimizedLoops and luau default to true.
var defaultProjectOptions = projectOptions{
	optimizedLoops: true,
	luau:           true,
}

// partialProjectOptions is a Partial<ProjectOptions>: nil fields are absent.
// Used for both the tsconfig `rbxts` key and the parsed argv, so the merge
// can mirror upstream's Object.assign semantics — in particular the
// load-bearing QUIRK that only --project has a yargs default ("DO NOT
// PROVIDE DEFAULTS BELOW HERE", CLI/commands/build.ts L62): booleans not
// passed on the CLI are ABSENT from argv and therefore never clobber values
// from the `rbxts` key.
type partialProjectOptions struct {
	includePath            *string
	rojo                   *string
	typeName               *string
	watch                  *bool
	usePolling             *bool
	verbose                *bool
	noInclude              *bool
	logTruthyChanges       *bool
	writeOnlyChanged       *bool
	writeTransformedFiles  *bool
	optimizedLoops         *bool
	allowCommentDirectives *bool
	luau                   *bool
}

// mergeProjectOptions ports the handler's Object.assign({},
// DEFAULT_PROJECT_OPTIONS, getTsConfigProjectOptions(tsConfigPath), argv)
// (CLI/commands/build.ts L125-130): later layers win, absent (nil) fields
// keep the earlier value.
func mergeProjectOptions(base projectOptions, layers ...*partialProjectOptions) projectOptions {
	out := base
	for _, layer := range layers {
		if layer == nil {
			continue
		}
		if layer.includePath != nil {
			out.includePath = *layer.includePath
		}
		if layer.rojo != nil {
			out.rojo = *layer.rojo
		}
		if layer.typeName != nil {
			out.typeName = *layer.typeName
		}
		if layer.watch != nil {
			out.watch = *layer.watch
		}
		if layer.usePolling != nil {
			out.usePolling = *layer.usePolling
		}
		if layer.verbose != nil {
			out.verbose = *layer.verbose
		}
		if layer.noInclude != nil {
			out.noInclude = *layer.noInclude
		}
		if layer.logTruthyChanges != nil {
			out.logTruthyChanges = *layer.logTruthyChanges
		}
		if layer.writeOnlyChanged != nil {
			out.writeOnlyChanged = *layer.writeOnlyChanged
		}
		if layer.writeTransformedFiles != nil {
			out.writeTransformedFiles = *layer.writeTransformedFiles
		}
		if layer.optimizedLoops != nil {
			out.optimizedLoops = *layer.optimizedLoops
		}
		if layer.allowCommentDirectives != nil {
			out.allowCommentDirectives = *layer.allowCommentDirectives
		}
		if layer.luau != nil {
			out.luau = *layer.luau
		}
	}
	return out
}

// findTsConfigPath ports findTsConfigPath (CLI/commands/build.ts L31-40):
// path.resolve the --project argument; if that is not an existing FILE, walk
// UP parent directories looking for tsconfig.json (ts.findConfigFile).
// Failure is upstream's exact CLIError text.
func findTsConfigPath(projectArg string) (string, error) {
	resolved, err := filepath.Abs(projectArg)
	if err != nil {
		return "", err
	}
	if st, err := os.Stat(resolved); err == nil && st.Mode().IsRegular() {
		return resolved, nil
	}
	for dir := resolved; ; {
		candidate := filepath.Join(dir, "tsconfig.json")
		if st, err := os.Stat(candidate); err == nil && st.Mode().IsRegular() {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("Unable to find tsconfig.json!")
		}
		dir = parent
	}
}

// readRbxtsOptions ports getTsConfigProjectOptions (CLI/commands/build.ts
// L22-29): read the FOUND tsconfig file raw (QUIRK verbatim: a RAW
// single-file read — `extends` is NOT followed for the `rbxts` key), parse as
// JSONC (ts.parseConfigFileTextToJson accepts comments/trailing commas), and
// return the top-level `rbxts` key — an undocumented way to persist any
// Partial<ProjectOptions> in tsconfig.json. Unreadable or unparsable input
// returns nil, like upstream's undefined.
func readRbxtsOptions(tsConfigPath string) *partialProjectOptions {
	data, err := os.ReadFile(tsConfigPath)
	if err != nil {
		return nil
	}
	var root struct {
		Rbxts *struct {
			IncludePath            *string `json:"includePath"`
			Rojo                   *string `json:"rojo"`
			Type                   *string `json:"type"`
			Watch                  *bool   `json:"watch"`
			UsePolling             *bool   `json:"usePolling"`
			Verbose                *bool   `json:"verbose"`
			NoInclude              *bool   `json:"noInclude"`
			LogTruthyChanges       *bool   `json:"logTruthyChanges"`
			WriteOnlyChanged       *bool   `json:"writeOnlyChanged"`
			WriteTransformedFiles  *bool   `json:"writeTransformedFiles"`
			OptimizedLoops         *bool   `json:"optimizedLoops"`
			AllowCommentDirectives *bool   `json:"allowCommentDirectives"`
			Luau                   *bool   `json:"luau"`
		} `json:"rbxts"`
	}
	if json.Unmarshal([]byte(compile.StripJSONC(string(data))), &root) != nil || root.Rbxts == nil {
		return nil
	}
	r := root.Rbxts
	return &partialProjectOptions{
		includePath:            r.IncludePath,
		rojo:                   r.Rojo,
		typeName:               r.Type,
		watch:                  r.Watch,
		usePolling:             r.UsePolling,
		verbose:                r.Verbose,
		noInclude:              r.NoInclude,
		logTruthyChanges:       r.LogTruthyChanges,
		writeOnlyChanged:       r.WriteOnlyChanged,
		writeTransformedFiles:  r.WriteTransformedFiles,
		optimizedLoops:         r.OptimizedLoops,
		allowCommentDirectives: r.AllowCommentDirectives,
		luau:                   r.Luau,
	}
}
