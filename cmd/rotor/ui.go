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
func (u *ui) watchBanner(n int) {
	g := u.s.Glyphs()
	fmt.Fprintf(u.w, "\n  %s  %s %s\n",
		u.s.Info(g.Watch),
		u.s.Muted(fmt.Sprintf("watching %s for changes", plural(n, "file"))),
		u.s.Muted("· Ctrl+C to exit"))
}

// watchIdle prints the post-build "watching for changes" line for build watch,
// where the watched set is the whole project tree rather than a fixed count.
func (u *ui) watchIdle() {
	g := u.s.Glyphs()
	fmt.Fprintf(u.w, "  %s  %s\n",
		u.s.Info(g.Watch),
		u.s.Muted("watching for changes · Ctrl+C to exit"))
}

// watchChange prints the rule + change-detected line on a rebuild.
func (u *ui) watchChange(file string) {
	fmt.Fprintf(u.w, "\n%s\n", u.s.Rule(64))
	fmt.Fprintf(u.w, "  %s %s %s\n\n",
		u.s.Muted(clock()),
		u.s.Info("change detected"),
		u.s.Muted(u.s.Glyphs().Dot+" "+filepath.Base(file)))
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
