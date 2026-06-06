package compile

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"rotor/tsgo/bundled"
	"rotor/tsgo/compiler"
	"rotor/tsgo/tsoptions"
	"rotor/tsgo/vfs/osvfs"
)

func TestSanitizeTSConfigStripsRejectedOptions(t *testing.T) {
	src := `{
	// rbxtsc requires these three options; tsgo (TS7) rejects them.
	"compilerOptions": {
		"downlevelIteration": true, /* removed in TS7 */
		"baseUrl": ".",
		"moduleResolution": "Node",
		"module": "commonjs",
		"strict": true,
	},
	"include": ["src"]
}`
	got := SanitizeTSConfig(src)

	var root map[string]any
	if err := json.Unmarshal([]byte(got), &root); err != nil {
		t.Fatalf("sanitized output is not valid JSON: %v\n%s", err, got)
	}
	co, ok := root["compilerOptions"].(map[string]any)
	if !ok {
		t.Fatalf("compilerOptions missing from sanitized output:\n%s", got)
	}
	if _, present := co["downlevelIteration"]; present {
		t.Error("downlevelIteration not stripped")
	}
	if _, present := co["baseUrl"]; present {
		t.Error("baseUrl not stripped")
	}
	if mr := co["moduleResolution"]; mr != "bundler" {
		t.Errorf("moduleResolution = %v, want %q", mr, "bundler")
	}
	if co["module"] != "commonjs" || co["strict"] != true {
		t.Errorf("unrelated options were altered: %v", co)
	}
	if inc, _ := root["include"].([]any); len(inc) != 1 || inc[0] != "src" {
		t.Errorf("include was altered: %v", root["include"])
	}
}

func TestSanitizeTSConfigLeavesValidConfigsAlone(t *testing.T) {
	src := `{"compilerOptions": {"module": "commonjs", "strict": true}}`
	got := SanitizeTSConfig(src)
	var root map[string]any
	if err := json.Unmarshal([]byte(got), &root); err != nil {
		t.Fatalf("sanitized output is not valid JSON: %v", err)
	}
	co := root["compilerOptions"].(map[string]any)
	if co["module"] != "commonjs" || co["strict"] != true {
		t.Errorf("options altered: %v", co)
	}
}

func TestSanitizeTSConfigMalformedJSONPassesThrough(t *testing.T) {
	src := `{ this is not json `
	if got := SanitizeTSConfig(src); got != src {
		t.Errorf("malformed input must pass through untouched for tsoptions to report; got %q", got)
	}
}

// TestFixtureProjectTypechecks is the acceptance test for the sanitizer: the
// rbxtsc fixture project (whose tsconfig.json carries all three rejected
// options) must produce ZERO config, options, and semantic diagnostics for
// src/01_literals.ts when loaded through the sanitized FS.
func TestFixtureProjectTypechecks(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("..", "..", "testdata", "diff", "project"))
	if err != nil {
		t.Fatal(err)
	}
	dir = filepath.ToSlash(dir)

	fs := SanitizeFS(bundled.WrapFS(osvfs.FS()))
	host := compiler.NewCompilerHost(dir, fs, bundled.LibPath(), nil, nil)

	parsed, configDiags := tsoptions.GetParsedCommandLineOfConfigFile(dir+"/tsconfig.json", nil, nil, host, nil)
	if len(configDiags) > 0 {
		for _, d := range configDiags {
			t.Errorf("config diagnostic: %v", d.String())
		}
		t.FailNow()
	}

	program := compiler.NewProgram(compiler.ProgramOptions{Host: host, Config: parsed})
	for _, d := range program.GetProgramDiagnostics() {
		t.Errorf("program diagnostic: %v", d.String())
	}

	sf := program.GetSourceFile(dir + "/src/01_literals.ts")
	if sf == nil {
		t.Fatal("01_literals.ts not in program")
	}
	for _, d := range program.GetSemanticDiagnostics(context.Background(), sf) {
		t.Errorf("semantic diagnostic: %v", d.String())
	}
}

// TestSanitizeFSOnlyTouchesTSConfig guards the wrapper's path filter.
func TestSanitizeFSOnlyTouchesTSConfig(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("..", "..", "testdata", "diff", "project"))
	if err != nil {
		t.Fatal(err)
	}
	dir = filepath.ToSlash(dir)

	inner := osvfs.FS()
	wrapped := SanitizeFS(inner)

	pkgPath := dir + "/package.json"
	want, ok1 := inner.ReadFile(pkgPath)
	got, ok2 := wrapped.ReadFile(pkgPath)
	if !ok1 || !ok2 || got != want {
		t.Error("non-tsconfig file was altered by SanitizeFS")
	}

	cfg, ok := wrapped.ReadFile(dir + "/tsconfig.json")
	if !ok {
		t.Fatal("tsconfig.json unreadable through SanitizeFS")
	}
	if strings.Contains(cfg, "downlevelIteration") {
		t.Error("tsconfig.json read through SanitizeFS still contains downlevelIteration")
	}
}
