package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"rotor/internal/compile"
)

// stripJSONCComments removes //-comment lines so JSONC (tsconfig.json) can be
// validated with encoding/json.
func stripJSONCComments(s string) string {
	lines := strings.Split(s, "\n")
	kept := lines[:0]
	for _, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "//") {
			continue
		}
		kept = append(kept, l)
	}
	return strings.Join(kept, "\n")
}

func mustReadFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestCmdInitGame(t *testing.T) {
	dir := t.TempDir()
	if code := cmdInit([]string{dir}); code != 0 {
		t.Fatalf("cmdInit exit code = %d", code)
	}

	for _, f := range []string{
		"package.json", "tsconfig.json", "default.project.json",
		".gitignore", "rotor.toml", "include/.gitkeep",
		"src/shared/module.ts", "src/server/main.server.ts", "src/client/main.client.ts",
	} {
		if !fileExists(filepath.Join(dir, f)) {
			t.Errorf("missing scaffolded file %s", f)
		}
	}
	for _, f := range []string{"src/shared/module.ts", "src/server/main.server.ts", "src/client/main.client.ts"} {
		if len(mustReadFile(t, filepath.Join(dir, f))) == 0 {
			t.Errorf("%s is empty", f)
		}
	}

	// package.json and default.project.json must be strict JSON.
	var pkg map[string]any
	if err := json.Unmarshal([]byte(mustReadFile(t, filepath.Join(dir, "package.json"))), &pkg); err != nil {
		t.Fatalf("package.json is not valid JSON: %v", err)
	}
	devDeps, _ := pkg["devDependencies"].(map[string]any)
	for _, dep := range []string{"@rbxts/compiler-types", "@rbxts/types", "typescript"} {
		v, _ := devDeps[dep].(string)
		if !strings.HasPrefix(v, "^") {
			t.Errorf("devDependencies[%q] = %q, want a loose ^ pin", dep, v)
		}
	}

	var proj map[string]any
	if err := json.Unmarshal([]byte(mustReadFile(t, filepath.Join(dir, "default.project.json"))), &proj); err != nil {
		t.Fatalf("default.project.json is not valid JSON: %v", err)
	}
	tree, _ := proj["tree"].(map[string]any)
	if tree["$className"] != "DataModel" {
		t.Errorf("tree.$className = %v, want DataModel", tree["$className"])
	}
	for _, svc := range []string{"ReplicatedStorage", "ServerScriptService", "StarterPlayer"} {
		if _, ok := tree[svc]; !ok {
			t.Errorf("tree missing %s", svc)
		}
	}

	// tsconfig.json is JSONC (jsx options commented out); after stripping
	// comments it must parse, and it must not configure baseUrl.
	tsconfig := mustReadFile(t, filepath.Join(dir, "tsconfig.json"))
	if strings.Contains(tsconfig, `"baseUrl"`) {
		t.Error("tsconfig.json must not set baseUrl")
	}
	if !strings.Contains(tsconfig, `// "jsx": "react"`) {
		t.Error("tsconfig.json should carry the commented-out jsx options")
	}
	var tsc map[string]any
	if err := json.Unmarshal([]byte(stripJSONCComments(tsconfig)), &tsc); err != nil {
		t.Fatalf("tsconfig.json (comments stripped) is not valid JSON: %v", err)
	}
	opts, _ := tsc["compilerOptions"].(map[string]any)
	if opts["outDir"] != "out" || opts["rootDir"] != "src" {
		t.Errorf("compilerOptions outDir/rootDir = %v/%v, want out/src", opts["outDir"], opts["rootDir"])
	}

	// rotor.schema.json is written whenever the schema is wired in.
	hasSchema := fileExists(filepath.Join(dir, "rotor.schema.json"))
	if want := configSchema != ""; hasSchema != want {
		t.Errorf("rotor.schema.json present = %v, want %v", hasSchema, want)
	}
	// rotor.toml's first line carries the taplo #:schema directive.
	toml := mustReadFile(t, filepath.Join(dir, "rotor.toml"))
	if !strings.HasPrefix(toml, "#:schema ./rotor.schema.json") {
		t.Errorf("rotor.toml should start with the #:schema directive:\n%s", toml)
	}

	// rotor-env.d.ts gives editors the $env macro types; the tsconfig include
	// must list it so tsserver actually picks it up.
	if got := mustReadFile(t, filepath.Join(dir, compile.EnvDeclFileName)); got != compile.EnvDeclFileText {
		t.Errorf("%s content differs from compile.EnvDeclFileText", compile.EnvDeclFileName)
	}
	if !strings.Contains(tsconfig, `"include": ["src", "rotor-env.d.ts"]`) {
		t.Error(`tsconfig include should list rotor-env.d.ts alongside src`)
	}
}

func TestCmdInitRefusesExistingProject(t *testing.T) {
	for _, marker := range []string{"package.json", "default.project.json"} {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, marker), []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
		if code := cmdInit([]string{dir}); code != 1 {
			t.Errorf("init over existing %s: exit %d, want 1", marker, code)
		}
	}
}

func TestCmdInitCreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "my-game")
	if code := cmdInit([]string{dir, "--template", "game"}); code != 0 {
		t.Fatalf("cmdInit exit code = %d", code)
	}
	if !fileExists(filepath.Join(dir, "package.json")) {
		t.Error("package.json not created in new directory")
	}
}

func TestCmdInitPackageTemplate(t *testing.T) {
	dir := t.TempDir()
	if code := cmdInit([]string{dir, "--template=package"}); code != 0 {
		t.Fatalf("cmdInit exit code = %d", code)
	}
	for _, f := range []string{"package.json", "tsconfig.json", "default.project.json", "src/init.ts", "rotor-env.d.ts"} {
		if !fileExists(filepath.Join(dir, f)) {
			t.Errorf("missing scaffolded file %s", f)
		}
	}
	if fileExists(filepath.Join(dir, "src/shared/module.ts")) {
		t.Error("package template should not scaffold the game src layout")
	}
	if !strings.Contains(mustReadFile(t, filepath.Join(dir, "tsconfig.json")), `"declaration": true`) {
		t.Error("package tsconfig should enable declaration output")
	}
}

func TestCmdInitPlainTemplate(t *testing.T) {
	dir := t.TempDir()
	if code := cmdInit([]string{dir, "--template", "plain"}); code != 0 {
		t.Fatalf("cmdInit exit code = %d", code)
	}
	for _, f := range []string{"default.project.json", "src/init.luau", "aftman.toml"} {
		if !fileExists(filepath.Join(dir, f)) {
			t.Errorf("missing scaffolded file %s", f)
		}
	}
	for _, f := range []string{"package.json", "tsconfig.json", "rotor.toml", "rotor.schema.json", "rotor-env.d.ts"} {
		if fileExists(filepath.Join(dir, f)) {
			t.Errorf("plain template should not write %s", f)
		}
	}
}

func TestCmdInitArgErrors(t *testing.T) {
	if code := cmdInit([]string{t.TempDir(), "--template", "library"}); code != 1 {
		t.Errorf("unknown template: exit %d, want 1", code)
	}
	if code := cmdInit([]string{"--bogus"}); code != 1 {
		t.Errorf("unknown flag: exit %d, want 1", code)
	}
	if code := cmdInit([]string{"a", "b"}); code != 1 {
		t.Errorf("extra positional: exit %d, want 1", code)
	}
	if code := cmdInit([]string{t.TempDir(), "--template"}); code != 1 {
		t.Errorf("--template without value: exit %d, want 1", code)
	}
}
