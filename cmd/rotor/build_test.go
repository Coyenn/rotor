package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestParseBuildArgs covers the rbxtsc-compatible flag surface
// (CLI/commands/build.ts L49-118).
func TestParseBuildArgs(t *testing.T) {
	t.Run("no args defaults project to dot with empty partial", func(t *testing.T) {
		got, err := parseBuildArgs(nil)
		if err != nil {
			t.Fatal(err)
		}
		if got.project != "." {
			t.Errorf("project = %q, want \".\"", got.project)
		}
		if got.opts != (partialProjectOptions{}) {
			t.Errorf("opts = %+v, want all-nil (no yargs defaults below --project)", got.opts)
		}
	})

	t.Run("usePolling without watch errors", func(t *testing.T) {
		_, err := parseBuildArgs([]string{"--usePolling"})
		if err == nil || !strings.Contains(err.Error(), "watch") {
			t.Errorf("err = %v, want implies-watch error", err)
		}
	})

	t.Run("usePolling with watch ok", func(t *testing.T) {
		got, err := parseBuildArgs([]string{"--usePolling", "-w"})
		if err != nil {
			t.Fatal(err)
		}
		if got.opts.usePolling == nil || !*got.opts.usePolling || got.opts.watch == nil || !*got.opts.watch {
			t.Errorf("opts = %+v", got.opts)
		}
	})

	t.Run("boolean negation forms", func(t *testing.T) {
		for _, args := range [][]string{{"--no-luau"}, {"--luau=false"}, {"--luau=0"}} {
			got, err := parseBuildArgs(args)
			if err != nil {
				t.Fatalf("%v: %v", args, err)
			}
			if got.opts.luau == nil || *got.opts.luau {
				t.Errorf("%v: luau = %v, want false", args, got.opts.luau)
			}
		}
		got, err := parseBuildArgs([]string{"--optimizedLoops=false"})
		if err != nil {
			t.Fatal(err)
		}
		if got.opts.optimizedLoops == nil || *got.opts.optimizedLoops {
			t.Error("--optimizedLoops=false not parsed")
		}
	})

	t.Run("plain boolean flags set true", func(t *testing.T) {
		got, err := parseBuildArgs([]string{"--verbose", "--noInclude", "--logTruthyChanges",
			"--writeOnlyChanged", "--writeTransformedFiles", "--allowCommentDirectives"})
		if err != nil {
			t.Fatal(err)
		}
		for name, p := range map[string]*bool{
			"verbose":                got.opts.verbose,
			"noInclude":              got.opts.noInclude,
			"logTruthyChanges":       got.opts.logTruthyChanges,
			"writeOnlyChanged":       got.opts.writeOnlyChanged,
			"writeTransformedFiles":  got.opts.writeTransformedFiles,
			"allowCommentDirectives": got.opts.allowCommentDirectives,
		} {
			if p == nil || !*p {
				t.Errorf("--%s not parsed", name)
			}
		}
	})

	t.Run("type choices", func(t *testing.T) {
		got, err := parseBuildArgs([]string{"--type", "model"})
		if err != nil {
			t.Fatal(err)
		}
		if got.opts.typeName == nil || *got.opts.typeName != "model" {
			t.Errorf("type = %v", got.opts.typeName)
		}
		if _, err := parseBuildArgs([]string{"--type", "bogus"}); err == nil {
			t.Error("invalid --type accepted")
		}
		if _, err := parseBuildArgs([]string{"--type"}); err == nil {
			t.Error("--type with no value accepted (yargs choices reject \"\")")
		}
	})

	t.Run("string flag forms", func(t *testing.T) {
		got, err := parseBuildArgs([]string{"-p", "proj", "-i", "inc", "--rojo=custom.project.json"})
		if err != nil {
			t.Fatal(err)
		}
		if got.project != "proj" {
			t.Errorf("project = %q", got.project)
		}
		if got.opts.includePath == nil || *got.opts.includePath != "inc" {
			t.Errorf("includePath = %v", got.opts.includePath)
		}
		if got.opts.rojo == nil || *got.opts.rojo != "custom.project.json" {
			t.Errorf("rojo = %v", got.opts.rojo)
		}
	})

	t.Run("rojo with no value is empty string", func(t *testing.T) {
		// QUIRK: `--rojo` / `--rojo ""` yields "" which falls through to
		// Rojo config auto-discovery (createProjectData.ts L33-43).
		got, err := parseBuildArgs([]string{"--rojo", "--verbose"})
		if err != nil {
			t.Fatal(err)
		}
		if got.opts.rojo == nil || *got.opts.rojo != "" {
			t.Errorf("rojo = %v, want present-and-empty", got.opts.rojo)
		}
		if got.opts.verbose == nil || !*got.opts.verbose {
			t.Error("--verbose after valueless --rojo not parsed")
		}
	})

	t.Run("positional project path", func(t *testing.T) {
		got, err := parseBuildArgs([]string{"some/dir", "--verbose"})
		if err != nil {
			t.Fatal(err)
		}
		if got.project != "some/dir" {
			t.Errorf("project = %q", got.project)
		}
	})

	t.Run("positional plus --project errors", func(t *testing.T) {
		if _, err := parseBuildArgs([]string{"a", "-p", "b"}); err == nil {
			t.Error("conflicting project paths accepted")
		}
	})

	t.Run("unknown flag errors", func(t *testing.T) {
		if _, err := parseBuildArgs([]string{"--bogus"}); err == nil {
			t.Error("unknown flag accepted")
		}
	})
}

// TestUsageErrorsExitOne pins the Phase 4 exit-code policy change: usage
// errors exit 1, matching upstream rbxtsc (CLI/cli.ts L30-35), not rotor's
// former 2.
func TestUsageErrorsExitOne(t *testing.T) {
	if got := run([]string{"frobnicate"}); got != 1 {
		t.Errorf("unknown command exit = %d, want 1", got)
	}
	if got := cmdBuild([]string{"--bogus"}); got != 1 {
		t.Errorf("unknown build flag exit = %d, want 1", got)
	}
	if got := cmdBuild([]string{"--usePolling"}); got != 1 {
		t.Errorf("--usePolling without --watch exit = %d, want 1", got)
	}
	if got := cmdCheck([]string{"--bogus"}); got != 1 {
		t.Errorf("unknown check flag exit = %d, want 1", got)
	}
}

func TestParseBuildArgsJSON(t *testing.T) {
	got, err := parseBuildArgs([]string{"--json", "."})
	if err != nil {
		t.Fatal(err)
	}
	if !got.jsonOut {
		t.Error("--json not parsed")
	}
}

// noLibGlobalStubs declares the fundamental global types the checker needs under
// noLib; mirrored from internal/compile's test helper so cmd-level tests build
// self-contained projects with no node_modules.
const noLibGlobalStubs = "declare function print(...params: Array<unknown>): void;\n" +
	"interface Array<T> {}\ninterface Boolean {}\ninterface CallableFunction {}\n" +
	"interface Function {}\ninterface IArguments {}\ninterface NewableFunction {}\n" +
	"interface Number {}\ninterface Object {}\ninterface RegExp {}\ninterface String {}\n"

// writeBuildableProject writes a minimal, self-contained Package project (a
// scoped name needs no Rojo config) that builds cleanly. mainSrc overrides
// src/main.ts when non-empty (e.g. to inject a diagnostic).
func writeBuildableProject(t *testing.T, mainSrc string) string {
	t.Helper()
	dir := t.TempDir()
	tsconfig := `{
	"compilerOptions": {
		"allowSyntheticDefaultImports": true,
		"module": "CommonJS",
		"moduleResolution": "Node",
		"noLib": true,
		"moduleDetection": "force",
		"strict": true,
		"target": "ESNext",
		"types": [],
		"typeRoots": ["node_modules/@rbxts"],
		"rootDir": "src",
		"outDir": "out"
	},
	"include": ["src"]
}`
	mustWrite(t, filepath.Join(dir, "tsconfig.json"), tsconfig)
	mustWrite(t, filepath.Join(dir, "package.json"), `{"name":"@scope/build-json-fixture"}`)
	mustWrite(t, filepath.Join(dir, "src", "globals.d.ts"), noLibGlobalStubs)
	if mainSrc == "" {
		mainSrc = "export {};\n"
	}
	mustWrite(t, filepath.Join(dir, "src", "main.ts"), mainSrc)
	return dir
}

// captureStdout runs fn with os.Stdout redirected to a pipe and returns what it
// wrote, plus fn's return value.
func captureStdout(t *testing.T, fn func() int) (string, int) {
	t.Helper()
	prev := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	code := fn()
	_ = w.Close()
	os.Stdout = prev
	data, _ := io.ReadAll(r)
	return string(data), code
}

func TestCmdBuildJSONClean(t *testing.T) {
	dir := writeBuildableProject(t, "")

	output, code := captureStdout(t, func() int {
		return cmdBuild([]string{"--json", dir})
	})
	if code != 0 {
		t.Fatalf("cmdBuild --json (clean) exit = %d, want 0; output:\n%s", code, output)
	}

	var res jsonResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &res); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput:\n%s", err, output)
	}
	if !res.OK {
		t.Errorf("ok = false on a clean build; diags: %+v", res.Diagnostics)
	}
	if res.Version == "" {
		t.Error("version is empty")
	}
	if res.Files <= 0 {
		t.Errorf("files = %d, want > 0", res.Files)
	}
	if res.Diagnostics == nil {
		t.Error("diagnostics must be [] not null")
	}
}

func TestCmdBuildJSONWithDiagnostic(t *testing.T) {
	// A type error: assign a number to a string-typed const.
	dir := writeBuildableProject(t, "export const s: string = 5;\n")

	output, code := captureStdout(t, func() int {
		return cmdBuild([]string{"--json", dir})
	})
	if code != 1 {
		t.Fatalf("cmdBuild --json (error) exit = %d, want 1; output:\n%s", code, output)
	}

	var res jsonResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &res); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput:\n%s", err, output)
	}
	if res.OK {
		t.Error("ok = true on a failing build")
	}
	if len(res.Diagnostics) == 0 {
		t.Error("expected at least one diagnostic")
	}
	if res.Diagnostics[0].Severity != "error" {
		t.Errorf("severity = %q, want error", res.Diagnostics[0].Severity)
	}
	if res.Diagnostics[0].Message == "" {
		t.Error("diagnostic message is empty")
	}
}
