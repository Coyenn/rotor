// Package logservice ports Shared/classes/LogService.ts and
// Shared/util/benchmark.ts: the upstream compiler's stdout channel, with
// partial-line tracking, a verbose gate, the yellow `Compiler Warning:`
// prefix, and the verbose benchmark line format (`name ( N ms )`).
//
// Like the upstream static class, state is package-level: one process, one
// log channel. Output is a variable so tests can capture it; color for the
// warning prefix is gated on NO_COLOR and TTY-ness of the writer (kleur's
// own enablement heuristic), never affecting bytes when piped.
package logservice

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// Verbose gates WriteLineIfVerbose and BenchmarkIfVerbose — set from the
// merged project options exactly once (`LogService.verbose =
// projectOptions.verbose === true`, CLI/commands/build.ts L132).
var Verbose bool

// Output is where everything is written (upstream process.stdout).
var Output io.Writer = os.Stdout

// partial tracks whether the last Write left an unterminated line
// (LogService.ts L5-9): a partial benchmark line gets a "\n" injected before
// any WriteLine so warnings never glue onto a pending ` ( N ms )` suffix.
var partial bool

// Write ports LogService.write (L7-10).
func Write(message string) {
	partial = !strings.HasSuffix(message, "\n")
	fmt.Fprint(Output, message)
}

// WriteLine ports LogService.writeLine (L12-19), including the
// partial-line "\n" injection.
func WriteLine(messages ...string) {
	if partial {
		Write("\n")
	}
	for _, message := range messages {
		Write(message + "\n")
	}
}

// WriteLineIfVerbose ports LogService.writeLineIfVerbose (L21-25).
func WriteLineIfVerbose(messages ...string) {
	if Verbose {
		WriteLine(messages...)
	}
}

// Warn ports LogService.warn (L27-29): kleur.yellow("Compiler Warning:") +
// " " + message. Warnings are NOT gated on Verbose and never fail a build.
func Warn(message string) {
	WriteLine(yellow("Compiler Warning:") + " " + message)
}

// BenchmarkIfVerbose ports benchmarkIfVerbose (Shared/util/benchmark.ts
// L18-24): under --verbose, write the name (no newline), run the callback,
// then append ` ( N ms )\n`; otherwise just run the callback.
func BenchmarkIfVerbose(name string, callback func()) {
	if !Verbose {
		callback()
		return
	}
	Write(name)
	start := time.Now()
	callback()
	Write(fmt.Sprintf(" ( %d ms )\n", time.Since(start).Milliseconds()))
}

// yellow wraps s in kleur.yellow's SGR codes (\x1b[33m ... \x1b[39m) when
// color is enabled for Output.
func yellow(s string) string {
	if useColor(Output) {
		return "\x1b[33m" + s + "\x1b[39m"
	}
	return s
}

// useColor mirrors the CLI's color gate (and kleur's enabled heuristic):
// NO_COLOR wins, otherwise color only when writing to a terminal.
func useColor(w io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
