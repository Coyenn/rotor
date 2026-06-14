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
// maxErrors caps the number of rendered frames across all groups (0 =
// unlimited); when capped, the footer notes how many were hidden.
func RenderGroups(groups []Group, o Options, maxErrors int) string {
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
			if maxErrors > 0 && shown >= maxErrors {
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
		fmt.Fprintf(&b, "  %s\n", s.Muted(fmt.Sprintf("…and %d more", hidden)))
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
	return fmt.Sprintf("  %s %s %s\n", s.ErrorBold("✗"),
		strings.Join(parts, " · "), s.Muted(fmt.Sprintf("in %s", plural(files, "file"))))
}

// plural renders "1 error" / "3 errors".
func plural(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}
