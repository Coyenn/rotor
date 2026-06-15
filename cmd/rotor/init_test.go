package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"rotor/internal/compile"
	"rotor/internal/config"
)

// must fails the surrounding test if err is non-nil (test setup helper).
func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

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

	// The schema is hosted (served from a raw-GitHub URL), so init must NOT
	// write a per-project rotor.schema.json.
	if fileExists(filepath.Join(dir, "rotor.schema.json")) {
		t.Error("init should not write a per-project rotor.schema.json (the schema is hosted)")
	}
	// rotor.toml's first line carries the taplo #:schema directive — the hosted URL.
	toml := mustReadFile(t, filepath.Join(dir, "rotor.toml"))
	if !strings.HasPrefix(toml, config.SchemaDirective) || !strings.HasPrefix(toml, "#:schema https://") {
		t.Errorf("rotor.toml should start with the hosted #:schema directive (%q):\n%s", config.SchemaDirective, toml)
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

func TestDetectTemplate(t *testing.T) {
	cases := []struct {
		name  string
		files map[string]string
		want  string
	}{
		{"plain", map[string]string{
			"default.project.json": `{"name":"x","tree":{"$path":"src"}}`,
		}, "plain"},
		{"package", map[string]string{
			"tsconfig.json":        `{"compilerOptions":{"declaration":true}}`,
			"default.project.json": `{"name":"x","tree":{"$path":"out"}}`,
		}, "package"},
		{"game", map[string]string{
			"tsconfig.json":        `{"compilerOptions":{}}`,
			"default.project.json": `{"name":"x","tree":{"$className":"DataModel"}}`,
		}, "game"},
		{"package via commented declaration stays game", map[string]string{
			"tsconfig.json": "{\n\t// \"declaration\": true,\n}",
		}, "game"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, content := range c.files {
				must(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
			}
			if got := detectTemplate(dir); got != c.want {
				t.Errorf("detectTemplate = %q, want %q", got, c.want)
			}
		})
	}
}

func TestAdoptFilesAndWrite(t *testing.T) {
	dir := t.TempDir()
	// An existing project plus a pre-existing env decl we must NOT clobber.
	must(t, os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(`{"compilerOptions":{}}`), 0o644))
	must(t, os.WriteFile(filepath.Join(dir, compile.EnvDeclFileName), []byte("// mine\n"), 0o644))

	var out bytes.Buffer
	if code := writeAdoptFiles(&out, dir, "game"); code != 0 {
		t.Fatalf("writeAdoptFiles = %d", code)
	}
	if !fileExists(filepath.Join(dir, config.ConfigFileName)) {
		t.Error("rotor.toml not created")
	}
	if fileExists(filepath.Join(dir, config.SchemaFileName)) {
		t.Error("adopt mode should not write a per-project rotor.schema.json (the schema is hosted)")
	}
	// The env decl must be kept verbatim.
	if got, _ := os.ReadFile(filepath.Join(dir, compile.EnvDeclFileName)); string(got) != "// mine\n" {
		t.Errorf("env decl clobbered: %q", got)
	}
	if !strings.Contains(out.String(), "exists, kept") {
		t.Errorf("expected a kept-file note:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "game project") {
		t.Errorf("expected the detected-template note:\n%s", out.String())
	}
}

func TestCmdInitAdoptsExistingProjectInsteadOfRefusing(t *testing.T) {
	for _, marker := range []string{"package.json", "tsconfig.json", "default.project.json"} {
		t.Run(marker, func(t *testing.T) {
			dir := t.TempDir()
			must(t, os.WriteFile(filepath.Join(dir, marker), []byte("{}"), 0o644))

			if code := cmdInit([]string{dir, "--yes"}); code != 0 {
				t.Fatalf("cmdInit over existing %s = %d (want adopt, exit 0)", marker, code)
			}
			if !fileExists(filepath.Join(dir, config.ConfigFileName)) {
				t.Errorf("adopt mode did not create rotor.toml for %s", marker)
			}
			// The pre-existing marker file must be left untouched.
			if got, _ := os.ReadFile(filepath.Join(dir, marker)); string(got) != "{}" {
				t.Errorf("%s was modified: %q", marker, got)
			}
		})
	}
}

func TestCmdInitConfigFlagAdoptsEmptyDir(t *testing.T) {
	dir := t.TempDir()
	if code := cmdInit([]string{dir, "--config"}); code != 0 {
		t.Fatalf("cmdInit --config = %d, want 0", code)
	}
	if !fileExists(filepath.Join(dir, config.ConfigFileName)) {
		t.Error("--config did not create rotor.toml")
	}
	// --config is config-only: it must not scaffold any source/project files.
	for _, f := range []string{"package.json", "tsconfig.json", "src/shared/module.ts"} {
		if fileExists(filepath.Join(dir, f)) {
			t.Errorf("--config wrote a non-config file %s", f)
		}
	}
}

func TestCmdInitAlreadyConfiguredIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	must(t, os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"x"}`), 0o644))
	must(t, os.WriteFile(filepath.Join(dir, config.ConfigFileName), []byte("# mine\n"), 0o644))

	if code := cmdInit([]string{dir, "--yes"}); code != 0 {
		t.Fatalf("cmdInit (already configured) = %d", code)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, config.ConfigFileName)); string(got) != "# mine\n" {
		t.Errorf("existing rotor.toml was overwritten: %q", got)
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
