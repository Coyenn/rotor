// Package term is rotor's terminal presentation layer: ANSI color/style
// helpers, a small set of semantic styles (success/error/warn/...), and glyphs
// for the CLI's human-facing output. Color is gated exactly once per writer on
// the usual heuristics — NO_COLOR disables, FORCE_COLOR forces, otherwise color
// is on only when the writer is an interactive terminal — so piped or
// redirected output stays plain ASCII and byte-stable.
//
// This package governs only rotor's OWN chrome (headers, summaries, progress).
// The upstream-compatible stdout channel (compiler warnings, --verbose
// benchmark lines) lives in internal/logservice and is deliberately left
// untouched so differential tests keep comparing exact bytes.
package term

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// SGR codes. Kept as small constants so the styler is allocation-light.
const (
	reset     = "\x1b[0m"
	bold      = "\x1b[1m"
	dim       = "\x1b[2m"
	italic    = "\x1b[3m"
	underline = "\x1b[4m"

	fgRed     = "\x1b[31m"
	fgGreen   = "\x1b[32m"
	fgYellow  = "\x1b[33m"
	fgBlue    = "\x1b[34m"
	fgMagenta = "\x1b[35m"
	fgCyan    = "\x1b[36m"
	fgGray    = "\x1b[90m"

	fgBrightGreen = "\x1b[92m"
	fgBrightCyan  = "\x1b[96m"
)

// Glyphs used across the CLI. These are common, well-supported code points;
// when color is disabled (e.g. a pipe) the ASCII fallbacks are used so logs
// stay readable in plain files.
type glyphSet struct {
	Check, Cross, Warn, Arrow, Bullet, Dot, Spark, Watch string
}

var (
	unicodeGlyphs = glyphSet{
		Check: "✓", Cross: "✗", Warn: "▲", Arrow: "→",
		Bullet: "•", Dot: "·", Spark: "⚡", Watch: "◷",
	}
	asciiGlyphs = glyphSet{
		Check: "ok", Cross: "x", Warn: "!", Arrow: "->",
		Bullet: "*", Dot: "-", Spark: "*", Watch: "~",
	}
)

// Styler renders styled text for a specific writer. Construct one with For and
// reuse it; every style method is a no-op passthrough when color is disabled.
type Styler struct {
	color bool
}

// For returns a Styler whose color enablement is decided once for w.
func For(w io.Writer) *Styler {
	return &Styler{color: ColorEnabled(w)}
}

// Color reports whether this Styler emits ANSI codes.
func (s *Styler) Color() bool { return s.color }

// Glyphs returns the glyph set appropriate to this Styler (unicode when color
// is on, ASCII fallbacks otherwise).
func (s *Styler) Glyphs() glyphSet {
	if s.color {
		return unicodeGlyphs
	}
	return asciiGlyphs
}

func (s *Styler) wrap(codes, text string) string {
	if !s.color || text == "" {
		return text
	}
	return codes + text + reset
}

// Semantic styles — the vocabulary the rest of the CLI speaks in.
func (s *Styler) Success(t string) string { return s.wrap(fgBrightGreen, t) }
func (s *Styler) Error(t string) string   { return s.wrap(fgRed, t) }
func (s *Styler) Warn(t string) string    { return s.wrap(fgYellow, t) }
func (s *Styler) Info(t string) string    { return s.wrap(fgBrightCyan, t) }
func (s *Styler) Accent(t string) string  { return s.wrap(fgMagenta, t) }
func (s *Styler) Muted(t string) string   { return s.wrap(fgGray, t) }

// Primitive styles, for composing one-off looks.
func (s *Styler) Bold(t string) string      { return s.wrap(bold, t) }
func (s *Styler) Dim(t string) string       { return s.wrap(dim, t) }
func (s *Styler) Italic(t string) string    { return s.wrap(italic, t) }
func (s *Styler) Underline(t string) string { return s.wrap(underline, t) }
func (s *Styler) Red(t string) string       { return s.wrap(fgRed, t) }
func (s *Styler) Green(t string) string     { return s.wrap(fgGreen, t) }
func (s *Styler) Yellow(t string) string    { return s.wrap(fgYellow, t) }
func (s *Styler) Blue(t string) string      { return s.wrap(fgBlue, t) }
func (s *Styler) Cyan(t string) string      { return s.wrap(fgCyan, t) }
func (s *Styler) Gray(t string) string      { return s.wrap(fgGray, t) }

// BoldColor applies bold + a semantic color in one wrap (e.g. a bold green
// headline) without doubling the reset sequence.
func (s *Styler) SuccessBold(t string) string { return s.wrap(bold+fgBrightGreen, t) }
func (s *Styler) ErrorBold(t string) string   { return s.wrap(bold+fgRed, t) }
func (s *Styler) WarnBold(t string) string    { return s.wrap(bold+fgYellow, t) }

// ColorEnabled reports whether ANSI color should be written to w. NO_COLOR
// (any value) disables; FORCE_COLOR (any value) forces; otherwise color is on
// only when w is a character device (an interactive terminal). This mirrors the
// kleur/standard heuristic the rest of the toolchain already follows.
func ColorEnabled(w io.Writer) bool {
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return false
	}
	if _, ok := os.LookupEnv("FORCE_COLOR"); ok {
		enableWindowsVT(w)
		return true
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil || info.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	enableWindowsVT(w)
	return true
}

// Plain strips a leading run of ANSI SGR sequences for width calculations.
// (Used only by the few places that need to align on visible width.)
func VisibleLen(s string) int {
	n, inEsc := 0, false
	for _, r := range s {
		switch {
		case inEsc:
			if r == 'm' {
				inEsc = false
			}
		case r == '\x1b':
			inEsc = true
		default:
			n++
		}
	}
	return n
}

// Hyperlink wraps text in an OSC 8 terminal hyperlink when color is enabled,
// so file paths in supporting terminals are clickable. Falls back to plain
// text otherwise.
func (s *Styler) Hyperlink(uri, text string) string {
	if !s.color {
		return text
	}
	return fmt.Sprintf("\x1b]8;;%s\x1b\\%s\x1b]8;;\x1b\\", uri, text)
}

// Rule returns a horizontal rule of the given width using box-drawing when
// color is on, ASCII dashes otherwise.
func (s *Styler) Rule(width int) string {
	ch := "─"
	if !s.color {
		ch = "-"
	}
	return s.Muted(strings.Repeat(ch, width))
}
