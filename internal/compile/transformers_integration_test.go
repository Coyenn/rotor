package compile

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func transformersFixtureDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "testdata", "transformers", "project"))
	if _, err := os.Stat(filepath.Join(dir, "node_modules", "rbxts-transformer-flamework", "package.json")); err != nil {
		t.Skipf("transformers fixture dependencies not installed (run `bun install --no-save` in testdata/transformers/project): %v", err)
	}
	if _, err := exec.LookPath("node"); err != nil {
		t.Skipf("node not on PATH: %v", err)
	}
	return dir
}

// TestTransformersFixtureFlameworkAndEnv runs the full production plugin
// path: embedded sidecar extraction (no ROTOR_SIDECAR_PATH), typescript
// resolved from the fixture's node_modules, warm session, real packages.
func TestTransformersFixtureFlameworkAndEnv(t *testing.T) {
	dir := transformersFixtureDir(t)
	t.Cleanup(closeSidecarSessions)
	closeSidecarSessions()
	t.Setenv("ROTOR_SIDECAR_PATH", "")
	redirectUserCacheDir(t)
	t.Setenv("ROTOR_FIXTURE_API_URL", "https://env.example")

	if err := os.RemoveAll(filepath.Join(dir, "out")); err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(filepath.Join(dir, "flamework.build"))

	result, diags, err := BuildProjectWithOptions(dir, ProjectOptions{})
	if err != nil {
		t.Fatalf("BuildProjectWithOptions: %v (diags: %v)", err, diags)
	}
	if len(diags) > 0 {
		t.Fatalf("diagnostics: %v", diags)
	}

	envOut := result.Outputs["out/shared/env.luau"]
	if !strings.Contains(envOut, "https://env.example") {
		t.Fatalf("rbxts-transform-env did not inline ROTOR_FIXTURE_API_URL:\n%s", envOut)
	}
	if strings.Contains(envOut, "$env") {
		t.Fatalf("rbxts-transform-env left $env macros in output:\n%s", envOut)
	}

	serviceOut := result.Outputs["out/server/services/test.service.luau"]
	if !strings.Contains(serviceOut, "identifier") || !strings.Contains(serviceOut, "defineMetadata") {
		t.Fatalf("rbxts-transformer-flamework did not inject identifier metadata:\n%s", serviceOut)
	}

	mainOut := result.Outputs["out/server/main.server.luau"]
	if strings.Contains(mainOut, `"src/server/services"`) {
		t.Fatalf("Flamework.addPaths was not rewritten:\n%s", mainOut)
	}
}
