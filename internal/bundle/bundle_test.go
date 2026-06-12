package bundle

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"rotor/internal/luau/cst"
)

func write(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func mustParse(t *testing.T, src string) {
	t.Helper()
	if _, diags := cst.Parse(src); len(diags) != 0 {
		t.Fatalf("bundle output does not parse: %s\n---\n%s", diags[0].Message, src)
	}
}

func TestBundleLinearGraph(t *testing.T) {
	dir := t.TempDir()
	entry := write(t, dir, "entry.luau", "local a = require(\"./a\")\nreturn a.value + 1\n")
	write(t, dir, "a.luau", "local b = require(\"./b\")\nreturn { value = b * 10 }\n")
	write(t, dir, "b.luau", "return 5\n")

	out, err := Bundle(entry)
	if err != nil {
		t.Fatalf("Bundle: %v", err)
	}
	mustParse(t, out)

	// three modules inlined, entry runs last
	for _, want := range []string{"impl_0(", "impl_1(", "impl_2(", "__ROTOR_BUNDLE.load_0()"} {
		if !strings.Contains(out, want) {
			t.Errorf("bundle missing %q", want)
		}
	}
	// no raw relative requires remain
	if strings.Contains(out, "require(\"./a\")") || strings.Contains(out, "require(\"./b\")") {
		t.Errorf("relative require was not rewritten:\n%s", out)
	}
}

func TestBundleInitFolder(t *testing.T) {
	dir := t.TempDir()
	entry := write(t, dir, "entry.luau", "return require(\"./mod\")\n")
	if err := os.Mkdir(filepath.Join(dir, "mod"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, dir, "mod/init.luau", "return 42\n")

	out, err := Bundle(entry)
	if err != nil {
		t.Fatalf("Bundle: %v", err)
	}
	mustParse(t, out)
	if strings.Contains(out, "require(\"./mod\")") {
		t.Errorf("init-folder require not rewritten:\n%s", out)
	}
}

func TestBundleExternalRequireLeftAlone(t *testing.T) {
	dir := t.TempDir()
	entry := write(t, dir, "entry.luau",
		"local svc = require(game:GetService(\"ReplicatedStorage\").Thing)\nlocal x = require(\"./missing\")\nreturn svc\n")

	out, err := Bundle(entry)
	if err != nil {
		t.Fatalf("Bundle: %v", err)
	}
	mustParse(t, out)
	// instance-path require and unresolved relative require both stay verbatim
	if !strings.Contains(out, "require(game:GetService(\"ReplicatedStorage\").Thing)") {
		t.Errorf("instance-path require should be left alone:\n%s", out)
	}
	if !strings.Contains(out, "require(\"./missing\")") {
		t.Errorf("unresolved require should be left alone:\n%s", out)
	}
}

// TestBundleRunsUnderLune proves the headline property — the bundle still runs and
// preserves module semantics (run-once caching, exports). Skips if lune is absent.
func TestBundleRunsUnderLune(t *testing.T) {
	lune, err := exec.LookPath("lune")
	if err != nil {
		t.Skip("lune not on PATH")
	}
	dir := t.TempDir()
	// b runs once (increments a shared upvalue via a counter module), a multiplies,
	// entry prints. If caching were broken, count would be 2.
	entry := write(t, dir, "entry.luau", "local a = require(\"./a\")\nlocal b1 = require(\"./b\")\nlocal b2 = require(\"./b\")\nprint(a.value + 1, b1 == b2)\n")
	write(t, dir, "a.luau", "local b = require(\"./b\")\nreturn { value = b.n * 10 }\n")
	write(t, dir, "b.luau", "return { n = 5 }\n")

	out, err := Bundle(entry)
	if err != nil {
		t.Fatalf("Bundle: %v", err)
	}
	bundlePath := filepath.Join(dir, "bundle.luau")
	if err := os.WriteFile(bundlePath, []byte(out), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(lune, "run", bundlePath)
	got, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("lune run failed: %v\noutput:\n%s\nbundle:\n%s", err, got, out)
	}
	// a.value = b.n*10 = 50, +1 = 51; b1==b2 proves single-instance caching.
	if !strings.Contains(string(got), "51") || !strings.Contains(string(got), "true") {
		t.Fatalf("unexpected bundle output: %q", got)
	}
}

func TestBundleCycleTerminates(t *testing.T) {
	dir := t.TempDir()
	entry := write(t, dir, "entry.luau", "local a = require(\"./a\")\nreturn a\n")
	write(t, dir, "a.luau", "local b = require(\"./b\")\nreturn { b = b }\n")
	write(t, dir, "b.luau", "local a = require(\"./a\")\nreturn { a = a }\n") // cycle a<->b

	out, err := Bundle(entry)
	if err != nil {
		t.Fatalf("Bundle on cyclic graph should still succeed: %v", err)
	}
	mustParse(t, out)
	// the recursive-require guard is present
	if !strings.Contains(out, "required recursively") {
		t.Errorf("missing recursive-require guard:\n%s", out)
	}
	for _, want := range []string{"impl_0(", "impl_1(", "impl_2("} {
		if !strings.Contains(out, want) {
			t.Errorf("cyclic bundle missing %q", want)
		}
	}
}
