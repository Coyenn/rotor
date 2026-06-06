// Package compile wires program creation, transformation, and rendering into
// the project-aware entry points (the Go analog of upstream
// Project/functions/compileFiles.ts): CompileProject for whole projects and
// CompileFile as the single-file fast path the per-fixture tests use.
package compile

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"rotor/internal/luau/render"
	"rotor/internal/transformer"
	"rotor/tsgo/ast"
)

// CompileFile compiles projectDir/relPath to Luau source text. It returns the
// rendered text, any diagnostics as strings (TypeScript config/option/
// semantic diagnostics, project-validation failures, or transformer
// diagnostics), and a hard error. When diagnostics are returned the text is
// empty — mirroring upstream compileFiles.ts, which bails before transforming
// on pre-emit errors and before rendering on transformer errors.
//
// CompileFile deliberately keeps a single-file fast path instead of wrapping
// CompileProject: it builds the same Program and project context (Rojo
// resolver, ProjectType, runtimeLibRbxPath validation) but transforms only
// the requested file, so per-fixture tests stay isolated and fast. The diff
// harness migrates to CompileProject (Phase 3a Task 6).
func CompileFile(projectDir, relPath string) (string, []string, error) {
	dir, program, diags, err := newProjectProgram(projectDir)
	if err != nil {
		return "", diags, err
	}
	pctx, diags, err := newProjectContext(dir, program)
	if err != nil {
		return "", diags, err
	}
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
	state.SetRojoContext(pctx.rojoContext, pctx.projectType)
	return transformAndRender(state)
}

// transformAndRender runs the transformer and renderer behind a recover
// boundary: the transformer panics on internal invariant violations (ported
// upstream asserts — missing symbols, prereq-stack misuse), and a user's
// source must surface as an error, never crash the process.
func transformAndRender(state *transformer.State) (text string, diags []string, err error) {
	defer func() {
		if r := recover(); r != nil {
			text = ""
			diags = nil
			err = fmt.Errorf("internal compiler error: %v", r)
		}
	}()

	luauAST := transformer.TransformSourceFile(state)

	hasErrors := state.Diags.HasErrors()
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
