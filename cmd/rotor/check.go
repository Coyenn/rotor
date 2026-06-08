package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"rotor/internal/compile"
	"rotor/tsgo/ast"
	"rotor/tsgo/bundled"
	"rotor/tsgo/compiler"
	"rotor/tsgo/diagnostics"
	"rotor/tsgo/diagnosticwriter"
	"rotor/tsgo/tsoptions"
	"rotor/tsgo/tspath"
	"rotor/tsgo/vfs/cachedvfs"
	"rotor/tsgo/vfs/osvfs"
)

func cmdCheck(args []string) int {
	watch := false
	path := ""
	for _, a := range args {
		switch a {
		case "-w", "--watch":
			watch = true
		case "-h", "--help":
			usage(os.Stdout)
			return 0
		default:
			if strings.HasPrefix(a, "-") {
				fmt.Fprintf(os.Stderr, "rotor check: unknown flag %q\n\n", a)
				usage(os.Stderr)
				return 1 // usage errors exit 1 (rbxtsc parity; see main.go)
			}
			if path != "" {
				fmt.Fprintf(os.Stderr, "rotor check: unexpected extra argument %q\n\n", a)
				usage(os.Stderr)
				return 1
			}
			path = a
		}
	}
	if path == "" {
		path = "."
	}

	dir, err := filepath.Abs(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "rotor check: cannot resolve path %q: %v\n", path, err)
		return 1
	}
	if info, statErr := os.Stat(dir); statErr != nil || !info.IsDir() {
		fmt.Fprintf(os.Stderr, "rotor check: %s is not a directory\n", dir)
		return 1
	}
	if _, statErr := os.Stat(filepath.Join(dir, "tsconfig.json")); statErr != nil {
		fmt.Fprintf(os.Stderr, "rotor check: no tsconfig.json found in %s\n", dir)
		return 1
	}

	// rbxts-style projects resolve all of their types (including the
	// runtime/global definitions) out of node_modules; missing packages
	// produce a wall of confusing diagnostics, so warn up front.
	if _, statErr := os.Stat(filepath.Join(dir, "package.json")); statErr == nil {
		if _, statErr := os.Stat(filepath.Join(dir, "node_modules")); statErr != nil {
			fmt.Fprintf(os.Stderr,
				"rotor check: warning: %s has a package.json but no node_modules — type packages (e.g. @rbxts/*) cannot be resolved; install dependencies first\n",
				dir)
		}
	}

	newUI(os.Stdout).banner(filepath.Base(dir))

	if watch {
		return runWatch(dir, os.Stdout)
	}
	res := runCheck(dir, os.Stdout)
	if res.errorCount > 0 {
		return 1
	}
	return 0
}

type checkResult struct {
	fileCount  int
	errorCount int
	elapsed    time.Duration
	watchFiles []string
}

// runCheck builds a fresh program for the project in dir, prints all
// diagnostics plus a summary line, and reports the file list to watch.
func runCheck(dir string, out io.Writer) checkResult {
	start := time.Now()
	slashDir := filepath.ToSlash(dir)

	// SanitizeFS rewrites rbxts-required tsconfig options that tsgo (TS7)
	// rejects (downlevelIteration, baseUrl, moduleResolution=node10) so
	// standard roblox-ts projects check cleanly — same wrapping as
	// compile.CompileFile.
	// Cache filesystem metadata (Stat/FileExists/Realpath) for the lifetime of
	// this check: module resolution re-stats overlapping node_modules paths
	// once per file, and a check never mutates its source tree. Same wrapper
	// tsgo's project host uses; see compile.newProjectProgram.
	fs := cachedvfs.From(compile.SanitizeFS(bundled.WrapFS(osvfs.FS())))
	host := compiler.NewCompilerHost(slashDir, fs, bundled.LibPath(), nil, nil)

	formatOpts := &diagnosticwriter.FormattingOptions{
		NewLine: "\n",
		ComparePathsOptions: tspath.ComparePathsOptions{
			CurrentDirectory:          slashDir,
			UseCaseSensitiveFileNames: fs.UseCaseSensitiveFileNames(),
		},
	}

	configPath := slashDir + "/tsconfig.json"
	parsed, configDiags := tsoptions.GetParsedCommandLineOfConfigFile(configPath, nil, nil, host, nil)
	if parsed == nil {
		// Unreadable/unparsable config: report what we have and stop.
		writeDiagnostics(out, configDiags, formatOpts)
		res := checkResult{
			errorCount: countErrors(configDiags),
			elapsed:    time.Since(start),
			watchFiles: []string{configPath},
		}
		printSummary(out, res)
		return res
	}
	formatOpts.Locale = parsed.Locale()

	program := compiler.NewProgram(compiler.ProgramOptions{
		Host:   host,
		Config: parsed,
	})

	// Same collection order as tsgo's own CLI: config parse, syntactic,
	// program (options), global, then semantic diagnostics. Semantic checking
	// is scoped to the project's own files, mirroring rbxtsc — upstream calls
	// getPreEmitDiagnostics(program, sourceFile) per compiled file
	// (compileFiles.ts:156), so node_modules type packages are resolved
	// against, never themselves checked. (The fixture @rbxts/types, for one,
	// has internal errors that tsc only avoids via skipLibCheck.)
	ctx := context.Background()
	semanticProjectFilesOnly := func(ctx context.Context, _ *ast.SourceFile) []*ast.Diagnostic {
		var out []*ast.Diagnostic
		for _, name := range parsed.FileNames() {
			if sf := program.GetSourceFile(name); sf != nil {
				out = append(out, program.GetSemanticDiagnostics(ctx, sf)...)
			}
		}
		return out
	}
	diags := compiler.GetDiagnosticsOfAnyProgram(ctx, program, nil, false,
		program.GetBindDiagnostics, semanticProjectFilesOnly)
	diags = compiler.SortAndDeduplicateDiagnostics(diags)

	writeDiagnostics(out, diags, formatOpts)

	res := checkResult{
		fileCount:  len(program.GetSourceFiles()),
		errorCount: countErrors(diags),
		elapsed:    time.Since(start),
		watchFiles: append([]string{configPath}, parsed.FileNames()...),
	}
	printSummary(out, res)
	return res
}

func printSummary(out io.Writer, res checkResult) {
	newUI(out).checkSummary(res.fileCount, res.errorCount, res.elapsed)
}

func writeDiagnostics(out io.Writer, diags []*ast.Diagnostic, formatOpts *diagnosticwriter.FormattingOptions) {
	if len(diags) == 0 {
		return
	}
	wrapped := diagnosticwriter.FromASTDiagnostics(diags)
	if useColor(out) {
		diagnosticwriter.FormatDiagnosticsWithColorAndContext(out, wrapped, formatOpts)
		fmt.Fprint(out, formatOpts.NewLine)
	} else {
		diagnosticwriter.WriteFormatDiagnostics(out, wrapped, formatOpts)
	}
	fmt.Fprint(out, formatOpts.NewLine)
}

func countErrors(diags []*ast.Diagnostic) int {
	n := 0
	for _, d := range diags {
		if d.Category() == diagnostics.CategoryError {
			n++
		}
	}
	return n
}

func useColor(out io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	f, ok := out.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
