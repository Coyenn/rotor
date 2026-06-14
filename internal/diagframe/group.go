package diagframe

import (
	"fmt"
	"sort"
	"strings"

	"rotor/internal/term"
)

// Group is one file's diagnostics ready to render.
type Group struct {
	Path   string
	Source string
	Lang   Language
	Spots  []Spot
}

// RenderGroups renders all groups (files sorted by path), then a summary footer.
// maxFrames caps the total number of rendered diagnostic frames across all
// groups — errors and warnings alike (0 = unlimited); when capped, the footer
// notes how many were hidden.
func RenderGroups(groups []Group, o Options, maxFrames int) string {
	sorted := append([]Group(nil), groups...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Path < sorted[j].Path })

	var b strings.Builder
	var errs, warns, shown, total, filesWithDiags int
	for _, g := range sorted {
		if len(g.Spots) > 0 {
			filesWithDiags++
		}
		var render []Spot
		for _, sp := range g.Spots {
			total++
			if sp.Severity == Warning {
				warns++
			} else {
				errs++
			}
			if maxFrames > 0 && shown >= maxFrames {
				continue
			}
			render = append(render, sp)
			shown++
		}
		if len(render) > 0 {
			b.WriteString(Render(g.Path, g.Source, g.Lang, render, o))
			b.WriteString("\n")
		}
	}

	s := stylerFor(o.Color)
	if hidden := total - shown; hidden > 0 {
		ell := "…"
		if !o.Color {
			ell = "..." // keep no-color output plain ASCII / byte-stable
		}
		fmt.Fprintf(&b, "  %s\n", s.Muted(fmt.Sprintf("%sand %d more", ell, hidden)))
	}
	b.WriteString(summaryLine(s, errs, warns, filesWithDiags))
	return b.String()
}

func summaryLine(s *term.Styler, errs, warns, files int) string {
	var parts []string
	if errs > 0 {
		parts = append(parts, s.Error(plural(errs, "error")))
	}
	if warns > 0 {
		parts = append(parts, s.Warn(plural(warns, "warning")))
	}
	if len(parts) == 0 {
		return ""
	}
	g := s.Glyphs() // unicode when color is on, ASCII fallbacks otherwise
	glyph := s.ErrorBold(g.Cross)
	if errs == 0 {
		glyph = s.WarnBold(g.Warn)
	}
	sep := " · "
	if !s.Color() {
		sep = ", " // keep no-color output plain ASCII / byte-stable
	}
	return fmt.Sprintf("  %s %s %s\n", glyph,
		strings.Join(parts, sep), s.Muted(fmt.Sprintf("in %s", plural(files, "file"))))
}

// plural renders "1 error" / "3 errors".
func plural(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}
