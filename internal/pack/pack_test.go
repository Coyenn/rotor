package pack

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"rotor/internal/luau/cst"
)

const sampleRbxmx = `<roblox version="4">
  <Item class="Folder" referent="0">
    <Properties><string name="Name">demo</string></Properties>
    <Item class="ModuleScript" referent="1">
      <Properties>
        <string name="Name">greet</string>
        <string name="Source"><![CDATA[return function(name) return "Hi " .. name end]]></string>
      </Properties>
    </Item>
    <Item class="StringValue" referent="2">
      <Properties>
        <string name="Name">note</string>
        <string name="Value"><![CDATA[hello]]></string>
      </Properties>
    </Item>
  </Item>
</roblox>`

func TestParseRbxmx(t *testing.T) {
	roots, err := ParseRbxmx([]byte(sampleRbxmx))
	if err != nil {
		t.Fatal(err)
	}
	if len(roots) != 1 {
		t.Fatalf("roots = %d", len(roots))
	}
	root := roots[0]
	if root.ClassName != "Folder" || root.Name != "demo" || len(root.Children) != 2 {
		t.Fatalf("root = %+v", root)
	}
	greet := root.Children[0]
	if greet.ClassName != "ModuleScript" || greet.Name != "greet" || !strings.Contains(greet.Source, "Hi ") {
		t.Fatalf("greet = %+v", greet)
	}
	note := root.Children[1]
	if note.ClassName != "StringValue" || note.Value != "hello" {
		t.Fatalf("note = %+v", note)
	}
}

func TestEmitLuauParses(t *testing.T) {
	roots, _ := ParseRbxmx([]byte(sampleRbxmx))
	src, err := EmitLuau(roots, "demo.greet")
	if err != nil {
		t.Fatal(err)
	}
	if _, diags := cst.Parse(src); len(diags) != 0 {
		t.Fatalf("emitted bundle does not parse: %s\n---\n%s", diags[0].Message, src)
	}
	// a bad entry path errors
	if _, err := EmitLuau(roots, "demo.nope"); err == nil {
		t.Fatalf("expected error for missing entry")
	}
}

// TestPackLuauRunsUnderLune is the headline proof: a Rojo project, packed to a single
// self-reconstructing Luau script, runs and resolves instance-path requires. Skips
// without rojo + lune.
func TestPackLuauRunsUnderLune(t *testing.T) {
	rojo, err := exec.LookPath("rojo")
	if err != nil {
		t.Skip("rojo not on PATH")
	}
	lune, err := exec.LookPath("lune")
	if err != nil {
		t.Skip("lune not on PATH")
	}
	_ = rojo

	dir := t.TempDir()
	mkdir(t, filepath.Join(dir, "src"))
	writeFile(t, filepath.Join(dir, "src", "greet.luau"), "return function(name) return \"Hi \" .. name end\n")
	writeFile(t, filepath.Join(dir, "src", "main.luau"), "return require(script.Parent.greet)(\"world\")\n")
	writeFile(t, filepath.Join(dir, "default.project.json"),
		"{ \"name\": \"demo\", \"tree\": { \"$className\": \"Folder\", \"src\": { \"$path\": \"src\" } } }\n")

	bundle, err := Pack(Options{Project: dir, Format: FormatLuau, Entry: "demo.src.main"})
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}
	writeFile(t, filepath.Join(dir, "bundle.luau"), string(bundle))

	// the bundle returns require(main); print it so we can observe the value.
	// lune resolves "./bundle" relative to the requiring file (run.luau in dir).
	writeFile(t, filepath.Join(dir, "run.luau"), "print(require(\"./bundle\"))\n")
	cmd := exec.Command(lune, "run", "run.luau")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("lune run failed: %v\n%s\n--- bundle ---\n%s", err, out, bundle)
	}
	if !strings.Contains(string(out), "Hi world") {
		t.Fatalf("unexpected output: %q", out)
	}
}

func mkdir(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
}

func writeFile(t *testing.T, p, content string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
