package main

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"rotor/internal/term"
)

// ui renders rotor's human-facing CLI chrome (headers, summaries, the watch
// banner) with color when the target is a terminal. It is intentionally
// separate from internal/logservice, which owns the upstream-compatible stdout
// channel (compiler warnings and --verbose benchmark lines) and stays
// byte-stable for differential tests.
type ui struct {
	w io.Writer
	s *term.Styler
}

func newUI(w io.Writer) *ui {
	return &ui{w: w, s: term.For(w)}
}

// banner prints the compact product header shown at the top of a command.
func (u *ui) banner(sub string) {
	g := u.s.Glyphs()
	head := u.s.Accent(u.s.Bold("rotor")) + " " + u.s.Muted("v"+version)
	if sub != "" {
		head += "  " + u.s.Muted(g.Dot) + "  " + u.s.Info(sub)
	}
	fmt.Fprintf(u.w, "\n  %s\n\n", head)
}

// buildSuccess prints the post-build result block.
func (u *ui) buildSuccess(total, written, unchanged int, elapsed time.Duration) {
	g := u.s.Glyphs()
	ms := elapsed.Milliseconds()

	headline := fmt.Sprintf("%s  %s %s",
		u.s.SuccessBold(g.Check),
		u.s.Bold(fmt.Sprintf("compiled %s", plural(total, "file"))),
		u.s.Muted(fmt.Sprintf("in %d ms", ms)),
	)

	// Detail line: written/unchanged · throughput. Each part carries its own
	// style, so the separators (joinDot) are the only muted chrome.
	parts := []string{u.s.Bold(itoa(written)) + u.s.Muted(" written")}
	if unchanged > 0 {
		parts = append(parts, u.s.Muted(fmt.Sprintf("%d unchanged", unchanged)))
	}
	if ms > 0 {
		rate := int(float64(total) / (float64(ms) / 1000.0))
		parts = append(parts, u.s.Muted(fmt.Sprintf("%d files/s", rate)))
	}

	fmt.Fprintf(u.w, "  %s\n", headline)
	fmt.Fprintf(u.w, "    %s\n\n", joinDot(u.s, parts))
}

// buildFailure prints a red failure header followed by the diagnostic lines.
func (u *ui) buildFailure(headline string, diags []string) {
	g := u.s.Glyphs()
	fmt.Fprintf(u.w, "  %s  %s\n", u.s.ErrorBold(g.Cross), u.s.Bold(headline))
	if len(diags) > 0 {
		fmt.Fprintln(u.w)
		for _, d := range diags {
			fmt.Fprintf(u.w, "    %s\n", d)
		}
	}
	fmt.Fprintln(u.w)
}

// warn prints a single warning line in rotor chrome (distinct from the
// upstream-format logservice.Warn used for compiler warnings).
func (u *ui) warn(msg string) {
	g := u.s.Glyphs()
	fmt.Fprintf(u.w, "  %s  %s\n", u.s.WarnBold(g.Warn), msg)
}

// checkSummary prints the `rotor check` result line.
func (u *ui) checkSummary(files int, errs int, elapsed time.Duration) {
	g := u.s.Glyphs()
	ms := elapsed.Milliseconds()
	if errs == 0 {
		fmt.Fprintf(u.w, "\n  %s  %s %s\n\n",
			u.s.SuccessBold(g.Check),
			u.s.Bold(fmt.Sprintf("checked %s", plural(files, "file"))),
			u.s.Muted(fmt.Sprintf("in %d ms · no errors", ms)))
		return
	}
	fmt.Fprintf(u.w, "\n  %s  %s %s\n\n",
		u.s.ErrorBold(g.Cross),
		u.s.Bold(fmt.Sprintf("checked %s", plural(files, "file"))),
		u.s.Muted(fmt.Sprintf("in %d ms · ", ms))+u.s.Error(plural(errs, "error")))
}

// watchBanner prints the "watching N files" idle line.
func (u *ui) watchBanner(n int, stats *watchStats) {
	g := u.s.Glyphs()
	parts := []string{u.s.Muted(fmt.Sprintf("watching %s for changes", plural(n, "file")))}
	parts = append(parts, u.watchStatParts(stats)...)
	parts = append(parts, u.s.Muted("Ctrl+C to exit"))
	fmt.Fprintf(u.w, "\n  %s  %s\n", u.s.Info(g.Watch), joinDot(u.s, parts))
}

// watchIdle prints the post-build "watching for changes" line for build watch,
// where the watched set is the whole project tree rather than a fixed count.
func (u *ui) watchIdle(stats *watchStats) {
	g := u.s.Glyphs()
	parts := []string{u.s.Muted("watching for changes")}
	parts = append(parts, u.watchStatParts(stats)...)
	parts = append(parts, u.s.Muted("Ctrl+C to exit"))
	fmt.Fprintf(u.w, "  %s  %s\n", u.s.Info(g.Watch), joinDot(u.s, parts))
}

// watchStatParts renders the rebuild counter and the build-time sparkline for
// the idle lines: `rebuild #4 · 142 ms ▂▁▇▂`. Empty until the first rebuild.
func (u *ui) watchStatParts(stats *watchStats) []string {
	if stats == nil || stats.builds <= 1 {
		return nil
	}
	parts := []string{u.s.Muted(fmt.Sprintf("rebuild #%d", stats.builds-1))}
	if n := len(stats.history); n > 0 {
		last := u.s.Muted(fmt.Sprintf("%d ms", stats.history[n-1].Milliseconds()))
		// The sparkline is terminal chrome only — in piped/redirected output
		// the ASCII ramp reads as noise, so it stays interactive-only.
		if n > 1 && u.s.Color() {
			values := make([]float64, n)
			for i, d := range stats.history {
				values[i] = float64(d.Milliseconds())
			}
			last += " " + u.s.Info(u.s.Spark(values))
		}
		parts = append(parts, last)
	}
	return parts
}

// watchChanges prints the rule + change-detected line on a rebuild, listing
// the settled batch of changed files (up to three basenames).
func (u *ui) watchChanges(paths []string) {
	names := make([]string, 0, 3)
	for i, p := range paths {
		if i == 3 {
			break
		}
		names = append(names, filepath.Base(p))
	}
	label := strings.Join(names, ", ")
	if extra := len(paths) - len(names); extra > 0 {
		label += fmt.Sprintf(", +%d more", extra)
	}
	fmt.Fprintf(u.w, "\n%s\n", u.s.Rule(64))
	fmt.Fprintf(u.w, "  %s %s %s\n\n",
		u.s.Muted(clock()),
		u.s.Info(plural(len(paths), "file")+" changed"),
		u.s.Muted(u.s.Glyphs().Dot+" "+label))
}

// doctorRow prints one `rotor doctor` check: a status glyph, the label, the
// muted detail, and (for warn/fail) an indented hint line.
func (u *ui) doctorRow(c doctorCheck) {
	g := u.s.Glyphs()
	var mark string
	switch c.status {
	case doctorOK:
		mark = u.s.SuccessBold(g.Check)
	case doctorInfo:
		mark = u.s.Muted(g.Dot)
	case doctorWarn:
		mark = u.s.WarnBold(g.Warn)
	case doctorFail:
		mark = u.s.ErrorBold(g.Cross)
	}
	fmt.Fprintf(u.w, "  %s  %s  %s\n", mark, u.s.Bold(c.label), u.s.Muted(c.detail))
	if c.hint != "" && c.status >= doctorWarn {
		fmt.Fprintf(u.w, "      %s %s\n", u.s.Muted(g.Arrow), u.s.Muted(c.hint))
	}
}

// doctorSummary prints the closing tally line for `rotor doctor`.
func (u *ui) doctorSummary(total, fails, warns int) {
	g := u.s.Glyphs()
	switch {
	case fails > 0:
		fmt.Fprintf(u.w, "\n  %s  %s %s\n\n", u.s.ErrorBold(g.Cross),
			u.s.Bold(plural(fails, "problem")+" found"),
			u.s.Muted(fmt.Sprintf("(%d checks, %d warnings)", total, warns)))
	case warns > 0:
		fmt.Fprintf(u.w, "\n  %s  %s %s\n\n", u.s.WarnBold(g.Warn),
			u.s.Bold("ready, with "+plural(warns, "warning")),
			u.s.Muted(fmt.Sprintf("(%d checks)", total)))
	default:
		fmt.Fprintf(u.w, "\n  %s  %s %s\n\n", u.s.SuccessBold(g.Check),
			u.s.Bold("everything looks good"),
			u.s.Muted(fmt.Sprintf("(%d checks)", total)))
	}
}

// --- small formatting helpers ---

func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

func itoa(n int) string { return fmt.Sprintf("%d", n) }

func clock() string { return time.Now().Format("15:04:05") }

// joinDot joins parts with a muted middot separator.
func joinDot(s *term.Styler, parts []string) string {
	return strings.Join(parts, " "+s.Glyphs().Dot+" ")
}
