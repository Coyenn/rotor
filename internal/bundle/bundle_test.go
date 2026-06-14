package bundle

import (
	"errors"
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

func TestBundleLuaurcAlias(t *testing.T) {
	dir := t.TempDir()
	// Bundle root is the entry file's directory (`dir`); .luaurc sits there.
	write(t, dir, ".luaurc", `{ "aliases": { "shared": "shared" } }`)
	entry := write(t, dir, "entry.luau", "local util = require(\"@shared/util\")\nreturn util.x\n")
	if err := os.Mkdir(filepath.Join(dir, "shared"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, dir, "shared/util.luau", "return { x = 7 }\n")

	out, err := Bundle(entry)
	if err != nil {
		t.Fatalf("Bundle: %v", err)
	}
	mustParse(t, out)
	if strings.Contains(out, "require(\"@shared/util\")") {
		t.Errorf("alias require should have been rewritten:\n%s", out)
	}
	// util module was inlined alongside the entry.
	if !strings.Contains(out, "impl_0(") || !strings.Contains(out, "impl_1(") {
		t.Errorf("alias bundle missing a module:\n%s", out)
	}
	if !strings.Contains(out, "x = 7") {
		t.Errorf("aliased module body not present:\n%s", out)
	}
}

func TestBundleUndefinedAliasLeftAlone(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, ".luaurc", `{ "aliases": { "shared": "shared" } }`)
	entry := write(t, dir, "entry.luau", "return require(\"@other/thing\")\n")

	out, err := Bundle(entry)
	if err != nil {
		t.Fatalf("Bundle: %v", err)
	}
	mustParse(t, out)
	if !strings.Contains(out, "require(\"@other/thing\")") {
		t.Errorf("undefined alias require should be left verbatim:\n%s", out)
	}
}

func TestBundleBrokenLuaurcSkipped(t *testing.T) {
	dir := t.TempDir()
	// Not strict JSON (JSON5 comment + trailing comma): aliases are skipped.
	write(t, dir, ".luaurc", "{\n  // a comment\n  \"aliases\": { \"shared\": \"shared\", },\n}\n")
	entry := write(t, dir, "entry.luau", "return require(\"@shared/util\")\n")
	if err := os.Mkdir(filepath.Join(dir, "shared"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, dir, "shared/util.luau", "return 1\n")

	out, err := Bundle(entry)
	if err != nil {
		t.Fatalf("Bundle should succeed despite broken .luaurc: %v", err)
	}
	mustParse(t, out)
	// Alias unresolved -> require left verbatim.
	if !strings.Contains(out, "require(\"@shared/util\")") {
		t.Errorf("broken .luaurc should leave alias require verbatim:\n%s", out)
	}
}

func TestBundleJSONEmbedding(t *testing.T) {
	dir := t.TempDir()
	entry := write(t, dir, "entry.luau", "local d = require(\"./data.json\")\nreturn d.name\n")
	write(t, dir, "data.json", `{
		"name": "rotor",
		"count": 3,
		"ratio": 1.5,
		"enabled": true,
		"missing": null,
		"tags": ["a", "b"],
		"weird key": 9,
		"nested": { "deep": "value" }
	}`)

	out, err := Bundle(entry)
	if err != nil {
		t.Fatalf("Bundle: %v", err)
	}
	mustParse(t, out)
	if strings.Contains(out, "require(\"./data.json\")") {
		t.Errorf("json require should have been rewritten:\n%s", out)
	}
	// Scalars and structure present in emitted Luau.
	for _, want := range []string{
		`name = "rotor"`, "count = 3", "ratio = 1.5", "enabled = true",
		`["weird key"] = 9`, "nested = {", `deep = "value"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("json bundle missing %q:\n%s", want, out)
		}
	}
	// null object value -> key dropped (documented behavior).
	if strings.Contains(out, "missing") {
		t.Errorf("null object value should drop the key:\n%s", out)
	}
}

func TestBundleTxtEmbedding(t *testing.T) {
	dir := t.TempDir()
	entry := write(t, dir, "entry.luau", "return require(\"./note.txt\")\n")
	// Content contains "]]" which must force a higher long-string bracket level.
	content := "hello ]] world\nsecond line"
	write(t, dir, "note.txt", content)

	out, err := Bundle(entry)
	if err != nil {
		t.Fatalf("Bundle: %v", err)
	}
	mustParse(t, out)
	if strings.Contains(out, "require(\"./note.txt\")") {
		t.Errorf("txt require should have been rewritten:\n%s", out)
	}
	// A plain [[...]] would be terminated early by the "]]" in the content, so the
	// encoder must have escalated to [=[...]=] (or higher).
	if !strings.Contains(out, "[=[") {
		t.Errorf("txt with ]] should use a higher bracket level:\n%s", out)
	}
	if !strings.Contains(out, "hello ]] world") {
		t.Errorf("txt content not embedded verbatim:\n%s", out)
	}
}

func TestBundleExcludeGlob(t *testing.T) {
	dir := t.TempDir()
	entry := write(t, dir, "entry.luau",
		"local r = require(\"./runtime/provided\")\nlocal k = require(\"./keep\")\nreturn { r, k }\n")
	if err := os.Mkdir(filepath.Join(dir, "runtime"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, dir, "runtime/provided.luau", "return 1\n")
	write(t, dir, "keep.luau", "return 2\n")

	out, err := BundleWith(entry, Options{Exclude: []string{"runtime/**"}})
	if err != nil {
		t.Fatalf("BundleWith: %v", err)
	}
	mustParse(t, out)
	// Excluded require stays verbatim; the non-excluded one is inlined.
	if !strings.Contains(out, "require(\"./runtime/provided\")") {
		t.Errorf("excluded require should be left verbatim:\n%s", out)
	}
	if strings.Contains(out, "require(\"./keep\")") {
		t.Errorf("non-excluded require should be inlined:\n%s", out)
	}
}

func TestBundleCombinedRunOnce(t *testing.T) {
	// Code + JSON + txt in one graph, JSON required twice, must dedupe to a single
	// cached module (one impl per resolved path).
	dir := t.TempDir()
	entry := write(t, dir, "entry.luau",
		"local cfg1 = require(\"./cfg.json\")\nlocal cfg2 = require(\"./cfg.json\")\nlocal txt = require(\"./readme.md\")\nlocal a = require(\"./a\")\nreturn { cfg1, cfg2, txt, a }\n")
	write(t, dir, "cfg.json", `{ "v": 1 }`)
	write(t, dir, "readme.md", "# Title")
	write(t, dir, "a.luau", "return require(\"./cfg.json\").v\n")

	out, err := Bundle(entry)
	if err != nil {
		t.Fatalf("Bundle: %v", err)
	}
	mustParse(t, out)
	// Modules: entry, cfg.json, readme.md, a = 4 distinct impls. cfg.json required
	// three times (entry x2 + a) but appears once.
	for _, want := range []string{"impl_0(", "impl_1(", "impl_2(", "impl_3("} {
		if !strings.Contains(out, want) {
			t.Errorf("combined bundle missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "impl_4(") {
		t.Errorf("cfg.json should be a single cached module, not duplicated:\n%s", out)
	}
	if c := strings.Count(out, "local function impl_"); c != 4 {
		t.Errorf("expected 4 modules, got %d:\n%s", c, out)
	}
}

func TestBundleDataRunsUnderLune(t *testing.T) {
	lune, err := exec.LookPath("lune")
	if err != nil {
		t.Skip("lune not on PATH")
	}
	dir := t.TempDir()
	entry := write(t, dir, "entry.luau",
		"local d1 = require(\"./data.json\")\nlocal d2 = require(\"./data.json\")\nlocal t = require(\"./note.txt\")\nprint(d1.count, d1 == d2, #t > 0)\n")
	write(t, dir, "data.json", `{ "count": 42, "tags": ["x"] }`)
	write(t, dir, "note.txt", "body ]] with bracket")

	out, err := Bundle(entry)
	if err != nil {
		t.Fatalf("Bundle: %v", err)
	}
	bundlePath := filepath.Join(dir, "bundle.luau")
	if err := os.WriteFile(bundlePath, []byte(out), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := exec.Command(lune, "run", bundlePath).CombinedOutput()
	if err != nil {
		t.Fatalf("lune run failed: %v\noutput:\n%s\nbundle:\n%s", err, got, out)
	}
	// count=42, d1==d2 proves data module is cached, txt non-empty.
	if !strings.Contains(string(got), "42") || !strings.Contains(string(got), "true") {
		t.Fatalf("unexpected data-bundle output: %q", got)
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

func TestBundleParseErrorTyped(t *testing.T) {
	dir := t.TempDir()
	// "1 2" adjacent integer literals is a Luau parse error (unexpected token).
	entry := write(t, dir, "entry.luau", "return 1 2\n")

	_, err := Bundle(entry)
	if err == nil {
		t.Fatal("Bundle on invalid Luau should return an error")
	}
	var pe *ParseError
	if !errors.As(err, &pe) {
		t.Fatalf("error is %T (%v), want *ParseError", err, err)
	}
	if pe.Path == "" {
		t.Error("ParseError.Path is empty")
	}
	if pe.Source == "" {
		t.Error("ParseError.Source is empty")
	}
	if pe.Diag.Message == "" {
		t.Error("ParseError.Diag.Message is empty")
	}
	// Error() must preserve the legacy one-line "path:line:col: message" format.
	legacy := pe.Error()
	if !strings.Contains(legacy, ":") || !strings.Contains(legacy, pe.Diag.Message) {
		t.Errorf("ParseError.Error() = %q, want path:line:col: message form", legacy)
	}
}
