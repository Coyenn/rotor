package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// readPackageJSON parses a package.json file into a generic map for assertions.
func readPackageJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("package.json is not valid JSON: %v", err)
	}
	return m
}

// withCwd runs fn with the process working directory set to dir (cmdAdd reads
// ./package.json), restoring it afterward.
func withCwd(t *testing.T, dir string, fn func()) {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(prev); err != nil {
			t.Fatal(err)
		}
	}()
	fn()
}

func TestCmdAddInsertsDependency(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "package.json"),
		"{\n  \"name\": \"demo\",\n  \"version\": \"0.1.0\"\n}\n")

	withCwd(t, dir, func() {
		if code := cmdAdd([]string{"@rbxts/services", "@rbxts/foo"}); code != 0 {
			t.Fatalf("cmdAdd exit = %d, want 0", code)
		}
	})

	pkg := readPackageJSON(t, filepath.Join(dir, "package.json"))
	deps, ok := pkg["dependencies"].(map[string]any)
	if !ok {
		t.Fatalf("dependencies missing or wrong type: %#v", pkg["dependencies"])
	}
	// Known stable pin mirrored from init.go.
	if deps["@rbxts/services"] != "^1.0.0" {
		t.Errorf("@rbxts/services = %v, want ^1.0.0", deps["@rbxts/services"])
	}
	// Unknown package gets a loose "*".
	if deps["@rbxts/foo"] != "*" {
		t.Errorf("@rbxts/foo = %v, want *", deps["@rbxts/foo"])
	}
	// Existing fields preserved.
	if pkg["name"] != "demo" || pkg["version"] != "0.1.0" {
		t.Errorf("existing fields clobbered: %#v", pkg)
	}
}

func TestCmdAddDev(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "package.json"), "{\n  \"name\": \"demo\"\n}\n")

	withCwd(t, dir, func() {
		if code := cmdAdd([]string{"--dev", "typescript@^5.5.0"}); code != 0 {
			t.Fatalf("cmdAdd --dev exit = %d, want 0", code)
		}
	})

	pkg := readPackageJSON(t, filepath.Join(dir, "package.json"))
	dev, ok := pkg["devDependencies"].(map[string]any)
	if !ok {
		t.Fatalf("devDependencies missing: %#v", pkg)
	}
	if dev["typescript"] != "^5.5.0" {
		t.Errorf("typescript = %v, want ^5.5.0 (explicit version honored)", dev["typescript"])
	}
	if _, present := pkg["dependencies"]; present {
		t.Error("--dev must not create a dependencies map")
	}
}

func TestCmdAddDedupes(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "package.json"),
		"{\n  \"name\": \"demo\",\n  \"dependencies\": {\n    \"@rbxts/services\": \"^1.2.3\"\n  }\n}\n")

	withCwd(t, dir, func() {
		if code := cmdAdd([]string{"@rbxts/services"}); code != 0 {
			t.Fatalf("cmdAdd exit = %d, want 0", code)
		}
	})

	pkg := readPackageJSON(t, filepath.Join(dir, "package.json"))
	deps := pkg["dependencies"].(map[string]any)
	// The pre-existing pin must NOT be overwritten by the default pin.
	if deps["@rbxts/services"] != "^1.2.3" {
		t.Errorf("@rbxts/services = %v, want ^1.2.3 (existing entry preserved, not overwritten)", deps["@rbxts/services"])
	}
}

func TestCmdAddRefusesWithoutPackageJSON(t *testing.T) {
	dir := t.TempDir()
	withCwd(t, dir, func() {
		if code := cmdAdd([]string{"@rbxts/services"}); code != 1 {
			t.Fatalf("cmdAdd without package.json exit = %d, want 1", code)
		}
	})
}

func TestCmdAddNoArgs(t *testing.T) {
	if code := cmdAdd(nil); code != 1 {
		t.Fatalf("cmdAdd with no packages exit = %d, want 1", code)
	}
}

func TestCmdAddPreservesIndent(t *testing.T) {
	dir := t.TempDir()
	// 4-space indented file; the rewrite should keep 4-space indentation.
	mustWrite(t, filepath.Join(dir, "package.json"),
		"{\n    \"name\": \"demo\"\n}\n")

	withCwd(t, dir, func() {
		if code := cmdAdd([]string{"@rbxts/foo"}); code != 0 {
			t.Fatalf("cmdAdd exit = %d, want 0", code)
		}
	})

	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "\n    \"dependencies\"") {
		t.Errorf("4-space indent not preserved:\n%s", data)
	}
	if !strings.HasSuffix(string(data), "\n") {
		t.Error("file must end with a trailing newline")
	}
}
