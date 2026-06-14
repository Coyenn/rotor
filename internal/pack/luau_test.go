package pack

import (
	"strings"
	"testing"
)

// TestEmitLuauSyntaxErrorIncludesLocation verifies that when a script node has a Luau
// syntax error, the embedded failure message in the packed artifact includes the
// module name AND a line:col location (e.g. "at 1:10").
func TestEmitLuauSyntaxErrorIncludesLocation(t *testing.T) {
	// "return 1 2" triggers: unexpected '2' at line 1, col 10.
	badNode := &Instance{
		ClassName: "ModuleScript",
		Name:      "badMod",
		Source:    "return 1 2",
	}
	root := &Instance{
		ClassName: "Folder",
		Name:      "root",
		Children:  []*Instance{badNode},
	}

	out, err := EmitLuau([]*Instance{root}, "")
	if err != nil {
		t.Fatalf("EmitLuau returned unexpected error: %v", err)
	}

	// The embedded error string must contain the module name.
	if !strings.Contains(out, "badMod") {
		t.Errorf("output does not contain module name %q\n---\n%s", "badMod", out)
	}
	// Must contain a line:col location in the form "at 1:".
	if !strings.Contains(out, "at 1:") {
		t.Errorf("output does not contain line:col location (expected \"at 1:\")\n---\n%s", out)
	}
	// Must still contain the original parser message.
	if !strings.Contains(out, "unexpected") {
		t.Errorf("output does not contain the original parser message\n---\n%s", out)
	}
}
