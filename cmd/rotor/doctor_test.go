package main

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestReadPackageVersion(t *testing.T) {
	nodeModules := t.TempDir()
	writeTestFile(t, filepath.Join(nodeModules, "@rbxts", "types", "package.json"),
		`{"name": "@rbxts/types", "version": "1.0.812"}`)

	version, ok := readPackageVersion(nodeModules, "@rbxts/types")
	if !ok || version != "1.0.812" {
		t.Fatalf("readPackageVersion = (%q, %v), want (1.0.812, true)", version, ok)
	}
	if _, ok := readPackageVersion(nodeModules, "@rbxts/compiler-types"); ok {
		t.Fatal("readPackageVersion reported a missing package as installed")
	}
}

func TestTsconfigTransformerPlugins(t *testing.T) {
	dir := t.TempDir()
	tsConfig := filepath.Join(dir, "tsconfig.json")
	writeTestFile(t, tsConfig, `{
		// JSONC comments must parse
		"compilerOptions": {
			"plugins": [
				{ "transform": "rbxts-transformer-flamework" },
				{ "name": "not-a-transformer" },
				{ "transform": "rbxts-transform-env", "verbose": true },
			],
		},
	}`)

	got := tsconfigTransformerPlugins(tsConfig)
	want := []string{"rbxts-transformer-flamework", "rbxts-transform-env"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tsconfigTransformerPlugins = %v, want %v", got, want)
	}

	if got := tsconfigTransformerPlugins(filepath.Join(dir, "missing.json")); got != nil {
		t.Fatalf("missing tsconfig should list no plugins, got %v", got)
	}
}

func TestRunDoctorMissingTsConfigFails(t *testing.T) {
	checks, _ := runDoctor(t.TempDir())
	if len(checks) != 1 || checks[0].status != doctorFail || checks[0].label != "tsconfig.json" {
		t.Fatalf("checks = %+v, want a single tsconfig.json failure", checks)
	}
}

func TestRunDoctorReportsProjectState(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "tsconfig.json"), `{"compilerOptions": {}}`)
	writeTestFile(t, filepath.Join(dir, "package.json"), `{"name": "fixture"}`)
	writeTestFile(t, filepath.Join(dir, "node_modules", "@rbxts", "compiler-types", "package.json"),
		`{"version": "3.0.0-types.0"}`)
	writeTestFile(t, filepath.Join(dir, "default.project.json"), `{"name": "fixture", "tree": {}}`)

	// Resolve symlinks (macOS /var vs /private/var style aliasing) so the
	// upward tsconfig search lands on the same path we wrote.
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		resolved = dir
	}

	checks, _ := runDoctor(resolved)
	byLabel := map[string]doctorCheck{}
	for _, c := range checks {
		byLabel[c.label] = c
	}

	for label, status := range map[string]doctorStatus{
		"tsconfig.json":         doctorOK,
		"node_modules":          doctorOK,
		"@rbxts/compiler-types": doctorOK,
		"@rbxts/types":          doctorFail, // not installed in the fixture
		"Rojo project":          doctorOK,
	} {
		c, ok := byLabel[label]
		if !ok {
			t.Errorf("missing check %q in %v", label, checks)
			continue
		}
		if c.status != status {
			t.Errorf("check %q status = %v, want %v (detail: %s)", label, c.status, status, c.detail)
		}
	}
	if c := byLabel["@rbxts/compiler-types"]; c.detail != "v3.0.0-types.0" {
		t.Errorf("compiler-types detail = %q, want version string", c.detail)
	}
	// No transformer plugins configured: no sidecar or typescript checks.
	if _, ok := byLabel["transformer sidecar"]; ok {
		t.Error("sidecar check should only run when transformer plugins are configured")
	}
}

func TestRunDoctorNodeModulesMissingFails(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "tsconfig.json"), `{}`)
	writeTestFile(t, filepath.Join(dir, "package.json"), `{"name": "fixture"}`)

	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		resolved = dir
	}
	checks, _ := runDoctor(resolved)
	found := false
	for _, c := range checks {
		if c.label == "node_modules" {
			found = true
			if c.status != doctorFail {
				t.Errorf("node_modules status = %v, want fail", c.status)
			}
		}
	}
	if !found {
		t.Fatalf("no node_modules check in %+v", checks)
	}
}
