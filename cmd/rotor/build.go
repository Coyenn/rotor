package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"rotor/internal/compile"
	"rotor/internal/transformer"
)

// projectTypeChoices are the upstream --type choices (CLI/commands/build.ts
// L98-101: ProjectType.Game | Model | Package).
var projectTypeChoices = map[string]transformer.ProjectType{
	string(transformer.ProjectTypeGame):    transformer.ProjectTypeGame,
	string(transformer.ProjectTypeModel):   transformer.ProjectTypeModel,
	string(transformer.ProjectTypePackage): transformer.ProjectTypePackage,
}

// cmdBuild is the compile-to-disk command: CompileProject over the given
// directory, outputs written under the project per the PathTranslator
// (tsconfig outDir), plus the runtime library copy into the include folder
// (upstream copyInclude.ts via internal/includefiles, controlled by
// --noInclude / --includePath like rbxtsc's CLI/commands/build.ts L77-80,
// L102-106) and the --type ProjectType override (build.ts L98-101). The rest
// of the rbxtsc build surface — watch, incremental, --luau flag, .d.ts emit —
// is later Phase 4 work.
func cmdBuild(args []string) int {
	path := ""
	noInclude := false
	includePath := ""
	var projectType transformer.ProjectType
	for i := 0; i < len(args); i++ {
		a := args[i]

		// --includePath/-i takes a value, as `--includePath <path>` or
		// `--includePath=<path>` (yargs accepts both; the -i alias is
		// upstream's, CLI/commands/build.ts L102-106).
		if a == "--includePath" || a == "-i" {
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "rotor build: %s requires a path argument\n\n", a)
				usage(os.Stderr)
				return 2
			}
			i++
			includePath = args[i]
			continue
		}
		if v, ok := strings.CutPrefix(a, "--includePath="); ok {
			includePath = v
			continue
		}

		// --type overrides ProjectType inference, as upstream
		// (CLI/commands/build.ts L98-101 feeding compileFiles.ts L80).
		// Accepts `--type <v>` and `--type=<v>`.
		typeValue := ""
		hasTypeValue := false
		switch {
		case a == "--type":
			if i+1 >= len(args) {
				fmt.Fprint(os.Stderr, "rotor build: --type requires a value (game, model, or package)\n\n")
				usage(os.Stderr)
				return 2
			}
			i++
			typeValue = args[i]
			hasTypeValue = true
		case strings.HasPrefix(a, "--type="):
			typeValue = strings.TrimPrefix(a, "--type=")
			hasTypeValue = true
		}
		if hasTypeValue {
			pt, ok := projectTypeChoices[typeValue]
			if !ok {
				fmt.Fprintf(os.Stderr, "rotor build: invalid --type %q (choices: game, model, package)\n\n", typeValue)
				usage(os.Stderr)
				return 2
			}
			projectType = pt
			continue
		}

		switch a {
		case "--noInclude":
			noInclude = true
		case "-h", "--help":
			usage(os.Stdout)
			return 0
		default:
			if strings.HasPrefix(a, "-") {
				fmt.Fprintf(os.Stderr, "rotor build: unknown flag %q\n\n", a)
				usage(os.Stderr)
				return 2
			}
			if path != "" {
				fmt.Fprintf(os.Stderr, "rotor build: unexpected extra argument %q\n\n", a)
				usage(os.Stderr)
				return 2
			}
			path = a
		}
	}
	if path == "" {
		path = "."
	}

	dir, err := filepath.Abs(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "rotor build: cannot resolve path %q: %v\n", path, err)
		return 2
	}
	if info, statErr := os.Stat(dir); statErr != nil || !info.IsDir() {
		fmt.Fprintf(os.Stderr, "rotor build: %s is not a directory\n", dir)
		return 2
	}
	if _, statErr := os.Stat(filepath.Join(dir, "tsconfig.json")); statErr != nil {
		fmt.Fprintf(os.Stderr, "rotor build: no tsconfig.json found in %s\n", dir)
		return 2
	}
	if _, statErr := os.Stat(filepath.Join(dir, "package.json")); statErr == nil {
		if _, statErr := os.Stat(filepath.Join(dir, "node_modules")); statErr != nil {
			fmt.Fprintf(os.Stderr,
				"rotor build: warning: %s has a package.json but no node_modules — type packages (e.g. @rbxts/*) cannot be resolved; install dependencies first\n",
				dir)
		}
	}

	// Real builds carry rotor's own header; the upstream-header default is
	// only load-bearing for differential byte-comparison in tests.
	transformer.HeaderComment = " Compiled with rotor v" + version

	start := time.Now()
	results, diags, err := compile.CompileProjectWithOptions(dir, compile.ProjectOptions{
		IncludePath:      includePath,
		EmitIncludeFiles: !noInclude,
		Type:             projectType,
	})
	if err != nil {
		for _, d := range diags {
			fmt.Fprintln(os.Stderr, d)
		}
		fmt.Fprintf(os.Stderr, "rotor build: %v\n", err)
		return 1
	}

	// Deterministic write order (CompileProject returns a map).
	outPaths := make([]string, 0, len(results))
	for relOut := range results {
		outPaths = append(outPaths, relOut)
	}
	sort.Strings(outPaths)

	for _, relOut := range outPaths {
		absOut := filepath.Join(dir, filepath.FromSlash(relOut))
		if err := os.MkdirAll(filepath.Dir(absOut), 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "rotor build: %v\n", err)
			return 1
		}
		if err := os.WriteFile(absOut, []byte(results[relOut]), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "rotor build: %v\n", err)
			return 1
		}
		fmt.Println(relOut)
	}

	fmt.Printf("compiled %d files in %d ms\n", len(outPaths), time.Since(start).Milliseconds())
	return 0
}
