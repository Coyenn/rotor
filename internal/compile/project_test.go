package compile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ----------------------------------------------------------------------------
// Runtime-lib emission, one test per ProjectType.
//
// Ground truth: src/_scratch_instanceof.ts —
//
//	declare class Foo {}
//	declare const inst: object;
//	const isFoo = inst instanceof Foo;
//	print(isFoo);
//
// compiled 2026-06-06 through testdata/diff/project with real rbxtsc 3.0.0
// (scratch cleaned up after), once per project type:
//   - `npx rbxtsc --type model` with the checked-in default.project.json,
//   - `npx rbxtsc --type package`,
//   - `npx rbxtsc` (type INFERRED as game) with the tree swapped to
//     {"$className":"DataModel","ReplicatedStorage":{"out":{"$path":"out"},
//     "include":{"$path":"include"},...}}.
//
// The testdata/runtimelib_* projects reproduce those setups self-contained
// (no @rbxts dependency: `print` is declared ambiently, which elides and
// changes no output bytes). `instanceof` is the one Phase-2b construct that
// sets UsesRuntimeLib without needing imports, so these are full end-to-end
// proofs of the runtime-lib require emission ahead of Task 4's TS.import.
// ----------------------------------------------------------------------------

func compileRuntimeLibProject(t *testing.T, name string) map[string]string {
	t.Helper()
	files, diags, err := CompileProject(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("CompileProject: %v (diags: %v)", err, diags)
	}
	if len(diags) > 0 {
		t.Fatalf("diagnostics: %v", diags)
	}
	return files
}

// Model: relative property chain via RojoResolver.relative — file rbxPath
// ["main"], RuntimeLib ["include","RuntimeLib"] (the project name is never in
// any rbxPath), so the chain is script.Parent.include.RuntimeLib. rbxtsc
// output verbatim.
func TestCompileProjectRuntimeLibModel(t *testing.T) {
	files := compileRuntimeLibProject(t, "runtimelib_model")
	want := "-- Compiled with roblox-ts v3.0.0\n" +
		"local TS = require(script.Parent.include.RuntimeLib)\n" +
		"local isFoo = TS.instanceof(inst, Foo)\n" +
		"print(isFoo)\n" +
		"return nil\n"
	if got := files["out/main.luau"]; got != want {
		t.Errorf("out/main.luau:\ngot:\n%s\nwant:\n%s", got, want)
	}
	if len(files) != 1 {
		t.Errorf("produced files = %d, want 1 (%v)", len(files), keys(files))
	}
}

// Game: absolute GetService + WaitForChild chain over the runtimeLibRbxPath
// ["ReplicatedStorage","include","RuntimeLib"] — the one place WaitForChild
// is emitted literally. rbxtsc output verbatim (type inferred from the
// DataModel tree, also covering the inferProjectType Game branch).
func TestCompileProjectRuntimeLibGame(t *testing.T) {
	files := compileRuntimeLibProject(t, "runtimelib_game")
	want := "-- Compiled with roblox-ts v3.0.0\n" +
		"local TS = require(game:GetService(\"ReplicatedStorage\"):WaitForChild(\"include\"):WaitForChild(\"RuntimeLib\"))\n" +
		"local isFoo = TS.instanceof(inst, Foo)\n" +
		"print(isFoo)\n" +
		"return nil\n"
	if got := files["out/main.luau"]; got != want {
		t.Errorf("out/main.luau:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

// Package: `local TS = _G[script]`; the scoped package.json name selects
// ProjectType.Package (inferProjectType's first branch), runtimeLibRbxPath
// stays nil, and no Rojo config is required (synthetic resolver over outDir).
// rbxtsc output verbatim.
func TestCompileProjectRuntimeLibPackage(t *testing.T) {
	files := compileRuntimeLibProject(t, "runtimelib_package")
	want := "-- Compiled with roblox-ts v3.0.0\n" +
		"local TS = _G[script]\n" +
		"local isFoo = TS.instanceof(inst, Foo)\n" +
		"print(isFoo)\n" +
		"return nil\n"
	if got := files["out/main.luau"]; got != want {
		t.Errorf("out/main.luau:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

// CompileProject over the differential fixture project must reproduce every
// golden byte-for-byte — the whole-project path emits exactly what the
// per-file path (and rbxtsc) does. Task 6 moves the diff harness onto this.
func TestCompileProjectFixtureParity(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	files, diags, err := CompileProject(filepath.Join(root, "testdata", "diff", "project"))
	if err != nil {
		t.Fatalf("CompileProject: %v (diags: %v)", err, diags)
	}
	if len(diags) > 0 {
		t.Fatalf("diagnostics: %v", diags)
	}

	goldens, err := filepath.Glob(filepath.Join(root, "testdata", "diff", "golden", "*.luau"))
	if err != nil || len(goldens) == 0 {
		t.Fatalf("no goldens found (%v)", err)
	}
	for _, goldenPath := range goldens {
		name := filepath.Base(goldenPath)
		want, err := os.ReadFile(goldenPath)
		if err != nil {
			t.Fatal(err)
		}
		got, ok := files["out/"+strings.TrimSuffix(name, ".luau")+".luau"]
		if !ok {
			t.Errorf("%s: missing from CompileProject output (%v)", name, keys(files))
			continue
		}
		if got != string(want) {
			t.Errorf("%s: differs from golden", name)
		}
	}
	if len(files) != len(goldens) {
		t.Errorf("produced %d files, want %d", len(files), len(goldens))
	}
}

// ----------------------------------------------------------------------------
// The four plain-text emit failures (compileFiles.ts L82-98). Message text
// verified against rbxtsc 3.0.0 (the first one reproduced verbatim on
// 2026-06-06 by deleting the fixture's default.project.json; the rest ported
// from the reference).
// ----------------------------------------------------------------------------

// writeProject lays down a minimal compilable project; rojoConfig "" means no
// default.project.json.
func writeProject(t *testing.T, pkgName, rojoConfig string) string {
	t.Helper()
	dir := t.TempDir()
	tsconfig := `{
	"compilerOptions": {
		"module": "CommonJS",
		"moduleDetection": "force",
		"strict": true,
		"target": "ESNext",
		"types": [],
		"rootDir": "src",
		"outDir": "out"
	},
	"include": ["src"]
}`
	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(tsconfig), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"`+pkgName+`"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if rojoConfig != "" {
		if err := os.WriteFile(filepath.Join(dir, "default.project.json"), []byte(rojoConfig), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "src", "main.ts"), []byte("export {};\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestCompileProjectValidationFailures(t *testing.T) {
	tests := []struct {
		name       string
		rojoConfig string
		want       string
	}{
		{
			"non-package without rojo config",
			"",
			"Non-package projects must have a Rojo project file!",
		},
		{
			"include folder not covered",
			`{"name":"x","tree":{"$path":"out"}}`,
			"Rojo project contained no data for include folder!",
		},
		{
			// Network containers only apply to Game projects (getContainer
			// checks isGame), hence the DataModel tree.
			"runtime lib in server container",
			`{"name":"x","tree":{"$className":"DataModel","ServerScriptService":{"out":{"$path":"out"},"include":{"$path":"include"}}}}`,
			"Runtime library cannot be in a server-only or client-only container!",
		},
		{
			// PluginDebugService is isolated but in neither network table
			// (StarterGui etc. would trip the network check first).
			"runtime lib in isolated container",
			`{"name":"x","tree":{"$className":"DataModel","PluginDebugService":{"include":{"$path":"include"}},"ReplicatedStorage":{"out":{"$path":"out"}}}}`,
			"Runtime library cannot be in an isolated container!",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := writeProject(t, "validation-fixture", tt.rojoConfig)
			_, diags, err := CompileProject(dir)
			if err == nil {
				t.Fatal("expected a hard error")
			}
			if len(diags) != 1 || diags[0] != tt.want {
				t.Errorf("diags = %v, want [%q]", diags, tt.want)
			}
		})
	}
}

// A file whose output path no Rojo $path covers raises noRojoData from
// createRuntimeLibImport (TransformState.ts:232-241) when the file needs the
// runtime lib. The project validation passes (include is covered) — the
// failure is per-file.
func TestCompileProjectNoRojoDataForFile(t *testing.T) {
	dir := writeProject(t, "norojodata-fixture",
		`{"name":"x","tree":{"$path":"out/sub","include":{"$path":"include"}}}`)
	src := "declare class Foo {}\ndeclare const inst: object;\nexport const isFoo = inst instanceof Foo;\n"
	if err := os.WriteFile(filepath.Join(dir, "src", "main.ts"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	_, diags, err := CompileProject(dir)
	if err == nil {
		t.Fatal("expected a hard error")
	}
	want := "Could not find Rojo data. There is no $path in your Rojo config that covers " +
		filepath.Join("out", "main.luau")
	if len(diags) != 1 || diags[0] != want {
		t.Errorf("diags = %v, want [%q]", diags, want)
	}
}

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
