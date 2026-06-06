// Package compile wires program creation, transformation, and rendering into
// a single per-file entry point (the Go analog of upstream
// Project/functions/compileFiles.ts, narrowed to one file).
package compile

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"rotor/internal/luau/render"
	"rotor/internal/transformer"
	"rotor/tsgo/ast"
	"rotor/tsgo/bundled"
	"rotor/tsgo/compiler"
	"rotor/tsgo/tsoptions"
	"rotor/tsgo/vfs/osvfs"
)

// CompileFile compiles projectDir/relPath to Luau source text. It returns the
// rendered text, any diagnostics as strings (TypeScript config/option/
// semantic diagnostics, or transformer diagnostics), and a hard error. When
// diagnostics are returned the text is empty — mirroring upstream
// compileFiles.ts, which bails before transforming on pre-emit errors and
// before rendering on transformer errors.
func CompileFile(projectDir, relPath string) (string, []string, error) {
	dir, err := filepath.Abs(projectDir)
	if err != nil {
		return "", nil, err
	}
	dir = filepath.ToSlash(dir)

	fs := SanitizeFS(bundled.WrapFS(osvfs.FS()))
	host := compiler.NewCompilerHost(dir, fs, bundled.LibPath(), nil, nil)

	configPath := dir + "/tsconfig.json"
	parsed, configDiags := tsoptions.GetParsedCommandLineOfConfigFile(configPath, nil, nil, host, nil)
	if len(configDiags) > 0 {
		return "", diagnosticStrings(configDiags), errors.New("compile: tsconfig.json has errors")
	}

	program := compiler.NewProgram(compiler.ProgramOptions{
		Host:   host,
		Config: parsed,
	})
	ctx := context.Background()

	filePath := dir + "/" + filepath.ToSlash(relPath)
	sourceFile := program.GetSourceFile(filePath)
	if sourceFile == nil {
		return "", nil, fmt.Errorf("compile: source file not in program: %s", filePath)
	}

	// Program-level option diagnostics (e.g. removed-option checks) plus this
	// file's semantic diagnostics; any of them fails the compile before
	// transforming, mirroring upstream's pre-emit bail (compileFiles.ts:151-156).
	tsDiags := program.GetProgramDiagnostics()
	tsDiags = append(tsDiags, program.GetSemanticDiagnostics(ctx, sourceFile)...)
	if len(tsDiags) > 0 {
		return "", diagnosticStrings(tsDiags), errors.New("compile: TypeScript diagnostics")
	}

	chk, release := program.GetTypeChecker(ctx)
	defer release()

	state := transformer.NewState(program, chk, sourceFile, transformer.NewDiagService(), transformer.NewMultiState())
	luauAST, err := transformer.TransformSourceFile(state)
	if err != nil {
		return "", nil, err
	}

	hasErrors := state.Diags.HasErrors()
	var diags []string
	for _, d := range state.Diags.Flush() {
		diags = append(diags, d.Message)
	}
	if hasErrors {
		// Upstream bails before rendering when the transformer reported
		// errors (compileFiles.ts:176-178).
		return "", diags, nil
	}

	return render.RenderAST(luauAST), diags, nil
}

func diagnosticStrings(diags []*ast.Diagnostic) []string {
	out := make([]string, len(diags))
	for i, d := range diags {
		out[i] = d.String()
	}
	return out
}
