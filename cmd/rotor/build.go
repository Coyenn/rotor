package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"rotor/internal/compile"
	"rotor/internal/logservice"
	"rotor/internal/transformer"
)

// projectTypeChoices are the upstream --type choices (CLI/commands/build.ts
// L98-101: ProjectType.Game | Model | Package).
var projectTypeChoices = map[string]transformer.ProjectType{
	string(transformer.ProjectTypeGame):    transformer.ProjectTypeGame,
	string(transformer.ProjectTypeModel):   transformer.ProjectTypeModel,
	string(transformer.ProjectTypePackage): transformer.ProjectTypePackage,
}

// buildArgs is the parsed `rotor build` argv: the project path (the only
// option with a default — "DO NOT PROVIDE DEFAULTS BELOW HERE",
// CLI/commands/build.ts L62) plus a Partial<ProjectOptions> of exactly the
// flags the user passed.
type buildArgs struct {
	project string
	opts    partialProjectOptions
	help    bool
	version bool
}

// parseBuildArgs parses the rbxtsc-compatible `build` flag surface
// (CLI/commands/build.ts L49-118). Booleans accept `--flag`, `--flag=bool`,
// and the yargs-17 negation `--no-flag`; strings accept `--flag value` and
// `--flag=value` (a missing value parses as "", like yargs — load-bearing
// for the `--rojo` empty-string fall-through quirk). `--usePolling` implies
// `--watch`: yargs errors when usePolling appears in argv without watch.
// As rotor sugar (kept from the earlier CLI), one positional argument is
// accepted as the project path.
func parseBuildArgs(args []string) (*buildArgs, error) {
	res := &buildArgs{project: "."} // yargs default: --project "."
	positional := ""
	projectSet := false

	boolTargets := func(name string) **bool {
		switch name {
		case "watch", "w":
			return &res.opts.watch
		case "usePolling":
			return &res.opts.usePolling
		case "verbose":
			return &res.opts.verbose
		case "noInclude":
			return &res.opts.noInclude
		case "logTruthyChanges":
			return &res.opts.logTruthyChanges
		case "writeOnlyChanged":
			return &res.opts.writeOnlyChanged
		case "writeTransformedFiles":
			return &res.opts.writeTransformedFiles
		case "optimizedLoops":
			return &res.opts.optimizedLoops
		case "allowCommentDirectives":
			return &res.opts.allowCommentDirectives
		case "luau":
			return &res.opts.luau
		}
		return nil
	}

	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "-h", "--help":
			res.help = true
			return res, nil
		case "-v", "--version":
			res.version = true
			return res, nil
		}

		if !strings.HasPrefix(a, "-") {
			if positional != "" {
				return nil, fmt.Errorf("unexpected extra argument %q", a)
			}
			positional = a
			continue
		}

		// Split --name=value / alias normalization.
		name := strings.TrimLeft(a, "-")
		value, hasValue := "", false
		if eq := strings.IndexByte(name, '='); eq >= 0 {
			value, name = name[eq+1:], name[:eq]
			hasValue = true
		}

		// takeValue consumes the next argv entry as a string value; a
		// missing/flag-like next token yields "" (yargs string options).
		takeValue := func() string {
			if hasValue {
				return value
			}
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				return args[i]
			}
			return ""
		}

		switch name {
		case "project", "p":
			res.project = takeValue()
			projectSet = true
			continue
		case "includePath", "i":
			v := takeValue()
			res.opts.includePath = &v
			continue
		case "rojo":
			v := takeValue()
			res.opts.rojo = &v
			continue
		case "type":
			v := takeValue()
			if _, ok := projectTypeChoices[v]; !ok {
				return nil, fmt.Errorf("invalid --type %q (choices: game, model, package)", v)
			}
			res.opts.typeName = &v
			continue
		}

		// Boolean flags: --flag / --flag=bool / --no-flag.
		negated := false
		if rest, ok := strings.CutPrefix(name, "no-"); ok {
			name, negated = rest, true
		}
		if target := boolTargets(name); target != nil {
			b := !negated
			if hasValue {
				switch value {
				case "true", "1":
					b = true
				case "false", "0":
					b = false
				default:
					return nil, fmt.Errorf("invalid boolean value %q for --%s", value, name)
				}
				if negated {
					b = !b
				}
			}
			*target = &b
			continue
		}

		return nil, fmt.Errorf("unknown flag %q", a)
	}

	if projectSet && positional != "" {
		return nil, fmt.Errorf("unexpected extra argument %q (project already set via --project)", positional)
	}
	if positional != "" {
		res.project = positional
	}

	// yargs `implies: "watch"` (build.ts L68-72): --usePolling present in
	// argv without --watch is a usage error.
	if res.opts.usePolling != nil && res.opts.watch == nil {
		return nil, errors.New("--usePolling requires --watch (usePolling implies watch)")
	}

	return res, nil
}

// cmdBuild is the compile-to-disk command, porting the rbxtsc build handler
// (CLI/commands/build.ts L120-167): find the tsconfig (file path or upward
// search), merge ProjectOptions (defaults < tsconfig `rbxts` key < argv),
// set LogService verbosity, then compile and write outputs.
//
// Flag wiring status: --type/--noInclude/--includePath/--rojo/--luau/
// --logTruthyChanges/--optimizedLoops/--allowCommentDirectives/--verbose are
// live; --writeOnlyChanged now runs inside the compile package's output
// pipeline; --watch/
// --usePolling drive the polling watch loop;
// --writeTransformedFiles is parsed and ignored (rbxtsc plugin debug output —
// out of v1 scope).
//
// Exit-code policy: usage errors exit 1, matching upstream
// (`.fail(...)` sets exitCode 1, CLI/cli.ts L30-35 — rotor's earlier exit 2
// convention was a documented divergence, removed in Phase 4).
func cmdBuild(args []string) int {
	parsed, err := parseBuildArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "rotor build: %v\n\n", err)
		usage(os.Stderr)
		return 1
	}
	if parsed.help {
		usage(os.Stdout)
		return 0
	}
	if parsed.version {
		fmt.Println(version)
		return 0
	}

	tsConfigPath, err := findTsConfigPath(parsed.project)
	if err != nil {
		fmt.Fprintf(os.Stderr, "rotor build: %v\n", err)
		return 1
	}

	// Merge order (build.ts L125-130): defaults < tsconfig `rbxts` key <
	// argv. Absent CLI booleans (nil) never clobber `rbxts` values.
	opts := mergeProjectOptions(defaultProjectOptions, readRbxtsOptions(tsConfigPath), &parsed.opts)

	// LogService.verbose = projectOptions.verbose === true (build.ts L132).
	logservice.Verbose = opts.verbose

	// Upstream projectPath = path.dirname(tsConfigPath)
	// (createProjectData.ts L13).
	dir := filepath.Dir(tsConfigPath)

	if opts.writeTransformedFiles {
		logservice.Warn("--writeTransformedFiles is not supported by rotor yet (rbxtsc transformer-plugin debug output; out of v1 scope) — ignoring")
	}
	if opts.watch {
		return runBuildWatch(dir, tsConfigPath, opts)
	}

	if _, statErr := os.Stat(filepath.Join(dir, "package.json")); statErr == nil {
		if _, statErr := os.Stat(filepath.Join(dir, "node_modules")); statErr != nil {
			fmt.Fprintf(os.Stderr,
				"rotor build: warning: %s has a package.json but no node_modules — type packages (e.g. @rbxts/*) cannot be resolved; install dependencies first\n",
				dir)
		}
	}

	result, diags, elapsed, err := runBuildOnce(dir, tsConfigPath, opts)
	if err != nil {
		for _, d := range diags {
			fmt.Fprintln(os.Stderr, d)
		}
		fmt.Fprintf(os.Stderr, "rotor build: %v\n", err)
		return 1
	}

	// rotor's own summary line (rbxtsc 3.0.0 prints no total — deliberate UX
	// addition, via LogService so partial-line tracking holds).
	logservice.WriteLine(fmt.Sprintf("compiled %d files (%d written) in %d ms",
		len(result.Outputs), len(result.EmittedFiles), elapsed.Milliseconds()))
	return 0
}

func runBuildOnce(dir, tsConfigPath string, opts projectOptions) (*compile.BuildResult, []string, time.Duration, error) {
	// Real builds carry rotor's own header; the upstream-header default is
	// only load-bearing for differential byte-comparison in tests.
	transformer.HeaderComment = " Compiled with rotor v" + version

	start := time.Now()
	result, diags, err := compile.BuildProjectWithOptions(dir, compile.ProjectOptions{
		TsConfigPath:           tsConfigPath,
		IncludePath:            opts.includePath,
		EmitIncludeFiles:       !opts.noInclude,
		Type:                   transformer.ProjectType(opts.typeName),
		RojoConfigPath:         opts.rojo,
		LogTruthyChanges:       opts.logTruthyChanges,
		AllowCommentDirectives: opts.allowCommentDirectives,
		NoOptimizedLoops:       !opts.optimizedLoops,
		LuaExtension:           !opts.luau,
		WriteOnlyChanged:       opts.writeOnlyChanged,
	})
	return result, diags, time.Since(start), err
}
