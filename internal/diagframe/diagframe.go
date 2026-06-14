// Package diagframe renders a compiler/lexer diagnostic as a source "code
// frame": the offending line(s) with a gutter, a severity-colored
// caret/underline, and reserved keywords highlighted. It is language-agnostic —
// callers pass byte offsets into the source plus a Language selecting the
// reserved-keyword set. Output is plain ASCII (byte-stable) unless Options.Color.
package diagframe

import (
	"strings"
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
