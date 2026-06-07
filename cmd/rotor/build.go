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

// cmdBuild is the minimal compile-to-disk command: CompileProject over the
// given directory, outputs written under the project per the PathTranslator
// (tsconfig outDir). The full rbxtsc build surface — include/ copying, watch,
// incremental, --type/--luau flags, .d.ts emit — is Phase 4; this exists so
// rotor's emit can be exercised on real projects today.
func cmdBuild(args []string) int {
	path := ""
	for _, a := range args {
		switch a {
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
	results, diags, err := compile.CompileProject(dir)
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
	fmt.Println("note: include/ (RuntimeLib.lua, Promise.lua) is not copied yet — Phase 4")
	return 0
}
