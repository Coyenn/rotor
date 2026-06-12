package sourcemap

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func fixtureProject(t *testing.T) string {
	t.Helper()
	return filepath.Join("testdata", "fixture", "default.project.json")
}

func readGolden(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "fixture.golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	// Guard against CRLF checkouts: the golden is a single LF-terminated line.
	return strings.ReplaceAll(string(data), "\r\n", "\n")
}

func TestBuildFixtureGolden(t *testing.T) {
	root, supported, err := Build(fixtureProject(t))
	if err != nil {
		t.Fatal(err)
	}
	if !supported {
		t.Fatal("fixture should be within the native subset")
	}
	data, err := Marshal(root)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(data), readGolden(t); got != want {
		t.Fatalf("sourcemap mismatch\n got: %s\nwant: %s", got, want)
	}
}

func TestGenerateFixtureGolden(t *testing.T) {
	// Generate over the project DIRECTORY (resolveProject finds the file).
	data, err := Generate(filepath.Join("testdata", "fixture"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(data), readGolden(t); got != want {
		t.Fatalf("sourcemap mismatch\n got: %s\nwant: %s", got, want)
	}
}

// writeProject lays out files (relative path -> content) under dir.
func writeProject(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		full := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRelativeForwardSlashPaths(t *testing.T) {
	// An absolute Windows temp dir exercises both relativization and slash
	// conversion.
	dir := t.TempDir()
	writeProject(t, dir, map[string]string{
		"default.project.json": `{"name":"t","tree":{"$path":"src"}}`,
		"src/init.luau":        "return {}",
		"src/sub/mod.luau":     "return {}",
	})
	data, err := Generate(dir)
	if err != nil {
		t.Fatal(err)
	}
	var root Node
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, data)
	}
	var walk func(n *Node)
	walk = func(n *Node) {
		for _, p := range n.FilePaths {
			if strings.Contains(p, "\\") {
				t.Errorf("filePath %q contains a backslash", p)
			}
			if filepath.IsAbs(p) || strings.Contains(p, ":") {
				t.Errorf("filePath %q is not project-relative", p)
			}
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(&root)
	// Root with $path: path-derived file first, project file appended last.
	if got, want := strings.Join(root.FilePaths, ","), "src/init.luau,default.project.json"; got != want {
		t.Fatalf("root filePaths = %q, want %q", got, want)
	}
	if len(root.Children) != 1 || root.Children[0].Name != "sub" ||
		len(root.Children[0].Children) != 1 ||
		root.Children[0].Children[0].FilePaths[0] != "src/sub/mod.luau" {
		t.Fatalf("unexpected children: %s", data)
	}
}

func TestDataModelServiceInference(t *testing.T) {
	// Mirrors a built rbxts-style game tree (no globIgnorePaths so it stays in
	// the native subset). Expected output captured from
	// `rojo sourcemap --include-non-scripts` (Rojo 7.6.1).
	dir := t.TempDir()
	writeProject(t, dir, map[string]string{
		"default.project.json": `{
			"name": "game",
			"tree": {
				"$className": "DataModel",
				"ReplicatedStorage": {
					"rbxts_include": { "$path": "include" },
					"TS": { "$path": "out/shared" }
				},
				"ServerScriptService": {
					"TS": { "$path": "out/server" }
				},
				"StarterPlayer": {
					"StarterPlayerScripts": {
						"TS": { "$path": "out/client" }
					}
				}
			}
		}`,
		"out/shared/module.luau":      "return {}",
		"out/shared/Zebra.luau":       "return {}",
		"out/shared/apple.luau":       "return {}",
		"out/server/main.server.luau": "print(1)",
		"out/client/main.client.luau": "print(2)",
	})
	if err := os.MkdirAll(filepath.Join(dir, "include"), 0o755); err != nil {
		t.Fatal(err)
	}

	root, supported, err := Build(filepath.Join(dir, "default.project.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !supported {
		t.Fatal("DataModel game tree should be within the native subset")
	}
	data, err := Marshal(root)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"name":"game","className":"DataModel","filePaths":["default.project.json"],"children":[{"name":"ReplicatedStorage","className":"ReplicatedStorage","children":[{"name":"TS","className":"Folder","children":[{"name":"Zebra","className":"ModuleScript","filePaths":["out/shared/Zebra.luau"]},{"name":"apple","className":"ModuleScript","filePaths":["out/shared/apple.luau"]},{"name":"module","className":"ModuleScript","filePaths":["out/shared/module.luau"]}]},{"name":"rbxts_include","className":"Folder"}]},{"name":"ServerScriptService","className":"ServerScriptService","children":[{"name":"TS","className":"Folder","children":[{"name":"main","className":"Script","filePaths":["out/server/main.server.luau"]}]}]},{"name":"StarterPlayer","className":"StarterPlayer","children":[{"name":"StarterPlayerScripts","className":"StarterPlayerScripts","children":[{"name":"TS","className":"Folder","children":[{"name":"main","className":"LocalScript","filePaths":["out/client/main.client.luau"]}]}]}]}]}` + "\n"
	if string(data) != want {
		t.Fatalf("sourcemap mismatch\n got: %s\nwant: %s", data, want)
	}
}

func TestUnsupportedFeaturesFallBack(t *testing.T) {
	cases := map[string]map[string]string{
		"meta.json": {
			"default.project.json": `{"name":"t","tree":{"$path":"src"}}`,
			"src/init.luau":        "return {}",
			"src/thing.meta.json":  `{"className":"Tool"}`,
		},
		"globIgnorePaths": {
			"default.project.json": `{"name":"t","globIgnorePaths":["**/*.spec.luau"],"tree":{"$path":"src"}}`,
			"src/init.luau":        "return {}",
		},
		"missing path": {
			"default.project.json": `{"name":"t","tree":{"$path":"out"}}`,
		},
		"nested project": {
			"default.project.json":         `{"name":"t","tree":{"$path":"src"}}`,
			"src/init.luau":                "return {}",
			"src/sub/default.project.json": `{"name":"n","tree":{"$path":"."}}`,
		},
	}
	for name, files := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			writeProject(t, dir, files)
			_, supported, err := Build(filepath.Join(dir, "default.project.json"))
			if err != nil {
				t.Fatal(err)
			}
			if supported {
				t.Fatalf("%s should be outside the native subset", name)
			}
		})
	}
}

// TestMatchesRojo verifies byte parity with the real `rojo sourcemap
// --include-non-scripts` for the fixture. Skipped when rojo is not on PATH.
func TestMatchesRojo(t *testing.T) {
	rojoBin, err := exec.LookPath("rojo")
	if err != nil {
		t.Skip("rojo not on PATH")
	}
	abs, err := filepath.Abs(fixtureProject(t))
	if err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(rojoBin, "sourcemap", "--include-non-scripts", abs).Output()
	if err != nil {
		t.Fatalf("rojo sourcemap: %v", err)
	}
	got, err := Generate(abs)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(out) {
		t.Fatalf("native output diverges from rojo\nnative: %s\n  rojo: %s", got, out)
	}
}
