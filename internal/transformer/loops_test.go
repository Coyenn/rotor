package transformer_test

import (
	"path/filepath"
	"strings"
	"testing"

	"rotor/internal/luau/render"
	"rotor/internal/transformer"
	"rotor/tsgo/ast"
)

// The expected text in every test below is byte-for-byte what rbxtsc 3.0.0
// emits for the same source (verified by compiling each file through
// testdata/diff/project; header and trailing `return nil` stripped — those
// belong to TransformSourceFile, not the statement lists under test).

// renderFileStatements transforms the whole file's statement list and renders
// it, failing on any diagnostics.
func renderFileStatements(t *testing.T, relPath string) string {
	t.Helper()
	s := buildState(t, filepath.Join("testdata", "control"), relPath)
	statements := transformer.TransformStatementList(s, s.SourceFile.AsNode(), s.SourceFile.Statements.Nodes, nil)
	if ds := s.Diags.Flush(); len(ds) != 0 {
		t.Fatalf("unexpected diagnostics: %v", ds)
	}
	return render.RenderAST(statements)
}

// TestWhileConditionWithPrereqs: a condition whose transform produces
// prereqs (`i++ < 3`) cannot live in the while header — it is re-evaluated
// every iteration via `while true do` + prereqs + `if not cond then break`.
func TestWhileConditionWithPrereqs(t *testing.T) {
	want := `local i = 0
while true do
	local _original = i
	i += 1
	if not (_original < 3) then
		break
	end
	print(i)
end
`
	if got := renderFileStatements(t, "src/while.ts"); got != want {
		t.Errorf("rendered output differs from rbxtsc:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

// TestOptimizedNumericForLiteralBound: `for (let i = 0; i < 10; i++)` meets
// every precondition of the optimized numeric-for pass; the `<` bound is
// offset by -1 and constant-folds (`10` -> `9`). The non-literal-bound
// variant (`i < limit` -> `limit - 1`) is covered by diff fixture 05_control.
func TestOptimizedNumericForLiteralBound(t *testing.T) {
	want := `local total = 0
for i = 0, 9 do
	total += i
end
print(total)
`
	if got := renderFileStatements(t, "src/optfor.ts"); got != want {
		t.Errorf("rendered output differs from rbxtsc:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

// TestGeneralForStatement: condition `i !== 5` disqualifies the optimized
// pass, taking the fallback while-loop desugaring — outer `do` scope, the
// `_shouldIncrement` guard (increment at the top of every iteration except
// the first), and the relocated condition check.
func TestGeneralForStatement(t *testing.T) {
	want := `do
	local i = 0
	local _shouldIncrement = false
	while true do
		if _shouldIncrement then
			i += 1
		else
			_shouldIncrement = true
		end
		if not (i ~= 5) then
			break
		end
		print(i)
	end
end
`
	if got := renderFileStatements(t, "src/genfor.ts"); got != want {
		t.Errorf("rendered output differs from rbxtsc:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

// TestBareReturnInNestedBlock: `return;` emits an explicit `return nil`
// (preserving JS `undefined`), and a free-standing block becomes `do ... end`.
// Functions are not transformable yet, so the function BODY's statement list
// is transformed directly (rbxtsc's body output is the comparison text).
func TestBareReturnInNestedBlock(t *testing.T) {
	s := buildState(t, filepath.Join("testdata", "control"), "src/return.ts")

	declaration := s.SourceFile.Statements.Nodes[0]
	if !ast.IsFunctionDeclaration(declaration) {
		t.Fatalf("expected FunctionDeclaration first, got %v", declaration.Kind)
	}
	body := declaration.AsFunctionDeclaration().Body
	statements := transformer.TransformStatementList(s, body, body.AsBlock().Statements.Nodes, nil)
	if ds := s.Diags.Flush(); len(ds) != 0 {
		t.Fatalf("unexpected diagnostics: %v", ds)
	}

	want := `do
	if value > 0 then
		return nil
	end
	print("nested")
end
print(value)
`
	if got := render.RenderAST(statements); got != want {
		t.Errorf("rendered output differs from rbxtsc:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

// TestThrowString: `throw "..."` becomes a plain `error("...")` call
// statement (no type checks upstream).
func TestThrowString(t *testing.T) {
	want := `local bad = math.random() > 2
if bad then
	error("something went wrong")
end
print("ok")
`
	if got := renderFileStatements(t, "src/throw.ts"); got != want {
		t.Errorf("rendered output differs from rbxtsc:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

// TestLoopClosureCapturePanics: a non-optimized for loop whose body contains
// a function-like capturing the loop variable needs the per-iteration
// let-capture copy machinery (Phase 3) — rotor must fail loudly, never emit
// wrong output.
func TestLoopClosureCapturePanics(t *testing.T) {
	s := buildState(t, filepath.Join("testdata", "control"), "src/capture.ts")

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected a panic for loop closure capture, got none")
		}
		message, ok := r.(string)
		if !ok || !strings.Contains(message, "loop closure capture not yet supported") {
			t.Errorf("unexpected panic value: %v", r)
		}
	}()
	transformer.TransformStatementList(s, s.SourceFile.AsNode(), s.SourceFile.Statements.Nodes, nil)
}
