// Package diagframe renders a compiler/lexer diagnostic as a source "code
// frame": the offending line(s) with a gutter, a severity-colored
// caret/underline, and reserved keywords highlighted. It is language-agnostic —
// callers pass byte offsets into the source plus a Language selecting the
// reserved-keyword set. Output is plain ASCII (byte-stable) unless Options.Color.
package diagframe

import (
	"strings"

	"rotor/internal/term"
)

// Severity selects the headline word and caret color.
type Severity int

const (
	Error Severity = iota
	Warning
)

// Language selects which reserved-keyword set is highlighted.
type Language int

const (
	Luau Language = iota
	TypeScript
)

// Spot is one diagnostic anchored to a byte span of the source.
type Spot struct {
	Offset   int      // byte offset of the span start into the source
	Len      int      // span length in bytes; the caret count is max(1, Len-on-line)
	Severity Severity
	Code     string   // optional, shown after the message: "TS2322", "noAny", ""
	Message  string   // primary message (no suggestion/more-info tail)
	Help     []string // suggestion / "More information:" lines, rendered as `help:`
}

// Options controls presentation. Color and Link are decided by the caller from
// the destination writer (term.ColorEnabled); with Color false the output is
// plain ASCII and byte-stable.
type Options struct {
	Context int  // context lines above/below the primary line (default 1)
	Color   bool // emit ANSI styling
	Link    bool // emit OSC 8 hyperlink on the locator (caller: Color && wanted)
}

// lineColAt returns the 1-based line and column (byte column) for a byte offset,
// plus the 0-based index of the line's first byte. The offset is clamped to
// [0, len(src)].
func lineColAt(src string, offset int) (line, col, lineStart int) {
	if offset < 0 {
		offset = 0
	}
	if offset > len(src) {
		offset = len(src)
	}
	line = 1
	lineStart = 0
	for i := 0; i < offset; i++ {
		if src[i] == '\n' {
			line++
			lineStart = i + 1
		}
	}
	return line, offset - lineStart + 1, lineStart
}

// lineText returns the text of the given 1-based line, with any trailing CR
// stripped (so CRLF sources render cleanly). Returns "" when line is out of
// range.
func lineText(src string, line int) string {
	if line < 1 {
		return ""
	}
	cur := 1
	start := 0
	for i := 0; i < len(src); i++ {
		if cur == line {
			break
		}
		if src[i] == '\n' {
			cur++
			start = i + 1
		}
	}
	if cur != line {
		return ""
	}
	end := start
	for end < len(src) && src[end] != '\n' {
		end++
	}
	return strings.TrimSuffix(src[start:end], "\r")
}

// luauKeywords is the 21 Luau reserved words (mirrors internal/luau's canonical
// set; a later drift test guards against divergence).
var luauKeywords = map[string]struct{}{
	"and": {}, "break": {}, "do": {}, "else": {}, "elseif": {}, "end": {},
	"false": {}, "for": {}, "function": {}, "if": {}, "in": {}, "local": {},
	"nil": {}, "not": {}, "or": {}, "repeat": {}, "return": {}, "then": {},
	"true": {}, "until": {}, "while": {},
}

// tsKeywords is the TypeScript/JavaScript reserved + contextual keyword set used
// for highlighting (presentation only — not a parser).
var tsKeywords = map[string]struct{}{
	"abstract": {}, "any": {}, "as": {}, "asserts": {}, "async": {}, "await": {},
	"boolean": {}, "break": {}, "case": {}, "catch": {}, "class": {}, "const": {},
	"continue": {}, "debugger": {}, "declare": {}, "default": {}, "delete": {},
	"do": {}, "else": {}, "enum": {}, "export": {}, "extends": {}, "false": {},
	"finally": {}, "for": {}, "from": {}, "function": {}, "if": {}, "implements": {},
	"import": {}, "in": {}, "infer": {}, "instanceof": {}, "interface": {},
	"keyof": {}, "let": {}, "namespace": {}, "never": {}, "new": {}, "null": {},
	"number": {}, "object": {}, "of": {}, "private": {}, "protected": {},
	"public": {}, "readonly": {}, "return": {}, "static": {}, "string": {},
	"super": {}, "switch": {}, "this": {}, "throw": {}, "true": {}, "try": {},
	"type": {}, "typeof": {}, "undefined": {}, "unknown": {}, "var": {}, "void": {},
	"while": {}, "yield": {},
}

func isKeyword(word string, lang Language) bool {
	if lang == TypeScript {
		_, ok := tsKeywords[word]
		return ok
	}
	_, ok := luauKeywords[word]
	return ok
}

func isIdentByte(c byte) bool {
	return c == '_' || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')
}

// highlightKeywords colors reserved words in one line of source. A no-color
// styler returns the line unchanged (identity), so uncolored output is
// byte-stable. The scan is word-boundary based (keywords inside strings/comments
// may be colored — accepted for the keywords-only tier).
func highlightKeywords(line string, lang Language, s *term.Styler) string {
	if !s.Color() {
		return line
	}
	var b strings.Builder
	i := 0
	for i < len(line) {
		c := line[i]
		if c == '_' || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
			j := i + 1
			for j < len(line) && isIdentByte(line[j]) {
				j++
			}
			word := line[i:j]
			if isKeyword(word, lang) {
				b.WriteString(s.Accent(word))
			} else {
				b.WriteString(word)
			}
			i = j
			continue
		}
		b.WriteByte(c)
		i++
	}
	return b.String()
}

// stylerFor builds a Styler with color forced on or off (the renderer's single
// source of truth for the Options.Color decision, and a test seam).
func stylerFor(color bool) *term.Styler {
	if color {
		return term.For(term.ForceColorWriter{})
	}
	return term.For(term.PlainWriter{})
}
