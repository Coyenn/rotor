package diagframe

import (
	"strings"
	"testing"

	"rotor/internal/term"
)

func TestLineColAt(t *testing.T) {
	src := "ab\ncde\nf"
	cases := []struct {
		off, wantLine, wantCol int
	}{
		{0, 1, 1},  // 'a'
		{1, 1, 2},  // 'b'
		{3, 2, 1},  // 'c'
		{6, 2, 4},  // newline after "cde" -> col past 'e'
		{7, 3, 1},  // 'f'
		{99, 3, 2}, // clamp past end
	}
	for _, c := range cases {
		line, col, _ := lineColAt(src, c.off)
		if line != c.wantLine || col != c.wantCol {
			t.Errorf("offset %d: got %d:%d, want %d:%d", c.off, line, col, c.wantLine, c.wantCol)
		}
	}
}

func TestLineTextStripsCR(t *testing.T) {
	src := "x = 1\r\ny = 2\r\n"
	got := lineText(src, 1)
	if got != "x = 1" {
		t.Errorf("lineText line 1 = %q, want %q", got, "x = 1")
	}
}

func TestHighlightKeywords_Luau(t *testing.T) {
	got := highlightKeywords("local function f()", Luau, stylerFor(true))
	if !strings.Contains(got, "\x1b[") {
		t.Fatalf("expected ANSI codes, got %q", got)
	}
	if stripANSI(got) != "local function f()" {
		t.Errorf("visible text changed: %q", stripANSI(got))
	}
}

func TestHighlightKeywords_NoColorIsIdentity(t *testing.T) {
	in := "const x = 1"
	if got := highlightKeywords(in, TypeScript, stylerFor(false)); got != in {
		t.Errorf("no-color highlight changed text: %q", got)
	}
}

func TestKeywordSets(t *testing.T) {
	if !isKeyword("function", Luau) || !isKeyword("function", TypeScript) {
		t.Error("`function` should be a keyword in both")
	}
	if isKeyword("print", Luau) {
		t.Error("`print` is a global, not a reserved keyword")
	}
	if !isKeyword("const", TypeScript) || isKeyword("const", Luau) {
		t.Error("`const` is TS-only")
	}
}

func stripANSI(s string) string {
	var b strings.Builder
	inEsc := false
	for _, r := range s {
		switch {
		case inEsc:
			if r == 'm' {
				inEsc = false
			}
		case r == '\x1b':
			inEsc = true
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

var _ = term.For // keep the term import referenced even if not used directly elsewhere

func TestRender_BasicNoColor(t *testing.T) {
	src := "x = 1\n"
	got := Render("a.luau", src, Luau, []Spot{{
		Offset: 0, Len: 1, Severity: Error, Message: "bad",
	}}, Options{Color: false})
	want := "error: bad\n" +
		"  --> a.luau:1:1\n" +
		"  |\n" +
		"1 | x = 1\n" +
		"  | ^ bad\n"
	if got != want {
		t.Errorf("frame mismatch:\n--- got ---\n%q\n--- want ---\n%q", got, want)
	}
}

func TestRender_HelpLines(t *testing.T) {
	src := "x = 1\n"
	got := Render("a.luau", src, Luau, []Spot{{
		Offset: 0, Len: 1, Severity: Error, Message: "bad", Help: []string{"do this instead"},
	}}, Options{Color: false})
	if !strings.Contains(got, "help: do this instead") {
		t.Errorf("missing help line:\n%s", got)
	}
}

func TestRender_EmptySourceFallsBack(t *testing.T) {
	got := Render("a.luau", "", Luau, []Spot{{Offset: 0, Message: "boom", Severity: Error}}, Options{})
	if got != "a.luau:1:1: boom\n" {
		t.Errorf("fallback = %q", got)
	}
}

func TestRender_TabExpansionCaretAligns(t *testing.T) {
	src := "\tbad\n" // one leading tab, then "bad"
	got := Render("a.luau", src, Luau, []Spot{{Offset: 1, Len: 3, Severity: Error, Message: "x"}}, Options{Color: false})
	// tab expands to 4 spaces; caret sits under "bad" (visual col 5).
	if !strings.Contains(got, "  |     ^^^ x") {
		t.Errorf("caret not aligned after tab expansion:\n%q", got)
	}
}
