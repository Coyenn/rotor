package transformer

import (
	"strings"
	"testing"
)

// The JsxText pipeline (fixup + entity decode + backslash doubling) is
// unreachable in oracle-able fixtures: @rbxts/react's ReactNode excludes
// string, so raw text children never typecheck (digest §1.3/§6.3). These
// tests pin the digest's recorded shapes instead — §3 cases 4/9/E are
// oracle-verified 2026-06-07 via a type-widened throwaway project (see
// docs/superpowers/research/phase3c-jsx-digest.md §3) — plus the worked
// examples from TS's fixupWhitespaceAndDecodeEntities.ts comments.

func TestFixupWhitespaceAndDecodeEntities(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		// digest §3 case 4: leading/trailing newline+indent trimmed
		{"case4 hello world", "\n\t\t\thello world\n\t\t\t", "hello world"},
		// digest §3 case 9: &amp; -> &, &nbsp; -> U+00A0, lines joined with " "
		{"case9 entities", "\n\t\t\tone &amp; two&nbsp;three\n\t\t\tline2\n\t\t", "one & two three line2"},
		// digest §3 case E: backslashes pass through the fixup untouched
		// (doubling happens at the transformJsxChildren call site)
		{"caseE backslash", "back\\slash", "back\\slash"},
		// single line: returned as-is (including surrounding spaces)
		{"single line kept", "  one line  ", "  one line  "},
		// whitespace-only with a line break: "" (TS returns undefined; the
		// Go port returns "", matching upstream's `?? ""` consumption)
		{"all whitespace multiline", "  \n  ", ""},
		// trimRight first line, trim middles, trimLeft last line, drop empty
		{"three lines", "a  \n  b  \n  c", "a b c"},
		// empty lines removed before joining
		{"empty middle line", "a\n   \nb", "a b"},
		// decimal and hex numeric entities
		{"numeric entities", "&#123;&#x41;", "{A"},
		// unknown entity passes through verbatim
		{"unknown entity", "&bogus;", "&bogus;"},
		// missing semicolon: not an entity
		{"no semicolon", "a &amp b", "a &amp b"},
	}
	for _, tt := range tests {
		if got := fixupWhitespaceAndDecodeEntities(tt.in); got != tt.want {
			t.Errorf("%s: fixupWhitespaceAndDecodeEntities(%q) = %q, want %q", tt.name, tt.in, got, tt.want)
		}
	}
}

func TestDecodeEntities(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"no entities", "no entities"},
		{"&quot;&amp;&apos;&lt;&gt;", "\"&'<>"},
		{"&nbsp;", " "},
		{"&hellip;", "…"},
		{"&#65;", "A"},
		{"&#x2603;", "☃"},
		{"&#xZZ;", "&#xZZ;"}, // invalid hex digits -> literal
		{"&#;", "&#;"},       // empty numeric -> literal
		{"&;", "&;"},         // empty name -> literal
		{"tail &", "tail &"}, // trailing ampersand, no semicolon
	}
	for _, tt := range tests {
		if got := decodeEntities(tt.in); got != tt.want {
			t.Errorf("decodeEntities(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// TestJsxTextBackslashDoubling pins the call-site composition for digest §3
// case E: source text `back\slash` becomes the Luau string VALUE
// `back\\slash`, which renderStringLiteral emits raw as "back\\slash".
func TestJsxTextBackslashDoubling(t *testing.T) {
	fixed := fixupWhitespaceAndDecodeEntities("back\\slash")
	doubled := strings.ReplaceAll(fixed, "\\", "\\\\")
	if doubled != `back\\slash` {
		t.Errorf("backslash doubling: got %q, want %q", doubled, `back\\slash`)
	}
}
