package conformance

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestDetectRuntimeToolsHonorsEnvOverrides(t *testing.T) {
	dir := t.TempDir()
	rojoPath := filepath.Join(dir, "rojo.exe")
	lunePath := filepath.Join(dir, "lune.exe")
	for _, path := range []string{rojoPath, lunePath} {
		if err := os.WriteFile(path, []byte("stub"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("ROTOR_ROJO_PATH", rojoPath)
	t.Setenv("ROTOR_LUNE_PATH", lunePath)

	tools := detectRuntimeTools()
	if tools.Rojo != rojoPath {
		t.Fatalf("Rojo = %q, want %q", tools.Rojo, rojoPath)
	}
	if tools.Lune != lunePath {
		t.Fatalf("Lune = %q, want %q", tools.Lune, lunePath)
	}
}

func TestRuntimeSuiteSkipReasonReportsMissingTools(t *testing.T) {
	reason := runtimeSuiteSkipReason(RuntimeTools{Rojo: "C:/tools/rojo.exe"})
	if !strings.Contains(reason, "lune") {
		t.Fatalf("skip reason missing lune: %q", reason)
	}
	if strings.Contains(reason, "rojo") {
		t.Fatalf("skip reason should not report rojo missing when present: %q", reason)
	}
	if !strings.Contains(reason, "ROTOR_LUNE_PATH") {
		t.Fatalf("skip reason missing env override hint: %q", reason)
	}
}

func TestRuntimeSuiteSourceRels(t *testing.T) {
	projectDir := filepath.Join(repoRoot(t), "testdata", "conformance", "project")
	got, err := runtimeSuiteSourceRels(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"main.server.ts",
		"services.d.ts",
		"tests/assignment.spec.ts",
		"tests/delete.spec.ts",
		"tests/roact.spec.tsx",
		"tests/roact_spread.spec.tsx",
		"tests/template.spec.ts",
	} {
		if !slices.Contains(got, want) {
			t.Fatalf("runtime sources missing %q in %v", want, got)
		}
	}
}

func TestDisabledBehavioralFixturesHaveReasons(t *testing.T) {
	for rel, reason := range DisabledBehavioralFixtures {
		if reason == "" {
			t.Fatalf("%s should have a behavioral skip reason", rel)
		}
	}
}

func TestRuntimeRojoConfigUsesDataModelTopology(t *testing.T) {
	config := runtimeRojoConfig()
	for _, want := range []string{
		`"$className": "DataModel"`,
		`"ServerScriptService"`,
		`"main": {`,
		`"out/main.server.luau"`,
		`"tests": {`,
		`"out/tests"`,
	} {
		if !strings.Contains(config, want) {
			t.Fatalf("runtime rojo config missing %q:\n%s", want, config)
		}
	}
}

func TestWithRuntimeRoactCompatInjectsJsxShim(t *testing.T) {
	input := "local Roact = {}\n\nreturn Roact\n"
	got, err := withRuntimeRoactCompat(input)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`rawset(Roact, "Fragment", Fragment)`,
		`rawset(Roact, "jsx", function(component, props, ...)`,
		`normalized[Roact.Event[eventName]] = listener`,
		`normalized[Roact.Change[propertyName]] = listener`,
		`element._rotorKey = key`,
		`return Roact`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("patched runtime missing %q:\n%s", want, got)
		}
	}
}

func TestWithRuntimeRoactCompatRejectsUnexpectedInitShape(t *testing.T) {
	if _, err := withRuntimeRoactCompat("local Roact = {}\n"); err == nil {
		t.Fatal("expected patching error for runtime without return Roact")
	}
}

func TestBehavioralSuite(t *testing.T) {
	tools := detectRuntimeTools()
	if tools.Rojo == "" || tools.Lune == "" {
		t.Skip(runtimeSuiteSkipReason(tools))
	}
	if err := runBehavioralSuite(repoRoot(t), tools); err != nil {
		t.Fatal(err)
	}
}
