package compile

import (
	"path/filepath"
	"testing"

	"rotor/internal/luau"
	"rotor/internal/transformer"
	"rotor/tsgo/ast"
)

// withStubDispatch wires a TransformStatement that emits nothing for every
// statement, so the pipeline around it (program creation, sanitizer, module
// symbol, export shapes, return-nil rule, header, rendering) can be exercised
// end-to-end before Task 6 lands the real dispatch.
func withStubDispatch(t *testing.T) {
	t.Helper()
	prev := transformer.TransformStatement
	transformer.TransformStatement = func(s *transformer.State, node *ast.Node) *luau.List[luau.Statement] {
		return luau.NewList[luau.Statement]()
	}
	t.Cleanup(func() { transformer.TransformStatement = prev })
}

func fixtureProjectDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("..", "..", "testdata", "diff", "project"))
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestCompileFilePipelineNoExports(t *testing.T) {
	withStubDispatch(t)
	got, diags, err := CompileFile(fixtureProjectDir(t), filepath.Join("src", "01_literals.ts"))
	if err != nil {
		t.Fatalf("CompileFile: %v", err)
	}
	if len(diags) > 0 {
		t.Fatalf("diagnostics: %v", diags)
	}
	// Statement transforms are stubbed out, so only the header and the
	// ModuleScript `return nil` remain.
	want := "-- Compiled with roblox-ts v3.0.0\nreturn nil\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestCompileFilePipelineExportShapes(t *testing.T) {
	withStubDispatch(t)
	got, diags, err := CompileFile(fixtureProjectDir(t), filepath.Join("src", "08_exports.ts"))
	if err != nil {
		t.Fatalf("CompileFile: %v", err)
	}
	if len(diags) > 0 {
		t.Fatalf("diagnostics: %v", diags)
	}
	// 08_exports has `export const constant` (immutable) + `export let
	// mutable` (mutable): the mutable export forces the exports-table shape —
	// `local exports = {}` up top, the immutable pair assigned at the bottom,
	// then `return exports`.
	want := "-- Compiled with roblox-ts v3.0.0\n" +
		"local exports = {}\n" +
		"exports.constant = constant\n" +
		"return exports\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestCompileFileMissingSource(t *testing.T) {
	withStubDispatch(t)
	_, _, err := CompileFile(fixtureProjectDir(t), filepath.Join("src", "does_not_exist.ts"))
	if err == nil {
		t.Fatal("expected error for missing source file")
	}
}
