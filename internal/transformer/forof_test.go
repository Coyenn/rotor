package transformer_test

import (
	"path/filepath"
	"strings"
	"testing"

	"rotor/internal/luau/render"
	"rotor/internal/transformer"
)

// The expected text in every render test below is byte-for-byte what rbxtsc
// 3.0.0 emits for the same source (verified via scratch files compiled
// through testdata/diff/project — see tools/oracle/oracle.ps1 for the
// technique). Diagnostic probes have no goldens: rbxtsc aborts compilation on
// them, so only the diagnostic itself is pinned.

func renderForOfFile(t *testing.T, relPath string) string {
	t.Helper()
	s := buildState(t, filepath.Join("testdata", "forof"), relPath)
	statements := transformer.TransformStatementList(s, s.SourceFile.AsNode(), s.SourceFile.Statements.Nodes, nil)
	if ds := s.Diags.Flush(); len(ds) != 0 {
		t.Fatalf("unexpected diagnostics: %v", ds)
	}
	return render.RenderAST(statements)
}

// forOfDiagnostics transforms a probe file and returns rotor's diagnostics.
// The probes iterate non-array types (Set unions, Iterable<T>, $range's
// Iterable<number> return), which are valid rbxtsc 3.0.0 input but trip a
// known TS5->TS7 checker divergence in tsgo: TS7 resolves the global
// `Iterable`/`IterableIterator` types with arity 3 (checker.go:1082-1085)
// while @rbxts/compiler-types declares the TS5 arity-1 shape, so tsgo cannot
// see the ecosystem's iteration globals and reports "can only be iterated
// through when using '--downlevelIteration'" (and the sanitizer must strip
// downlevelIteration — TS7 removed the option). Tolerate exactly that
// message; everything else still fails. Phase 3's Set/Map/generator builders
// need a real fix (compiler-types overlay or checker shim).
func forOfDiagnostics(t *testing.T, relPath string) []transformer.Diagnostic {
	t.Helper()
	s := buildStateTolerating(t, filepath.Join("testdata", "forof"), relPath, func(msg string) bool {
		return strings.Contains(msg, "can only be iterated through")
	})
	transformer.TransformStatementList(s, s.SourceFile.AsNode(), s.SourceFile.Statements.Nodes, nil)
	return s.Diags.Flush()
}

// TestForOfObjectPatternInitializer: `for (const { x } of objs)` routes
// through transformBindingName — the loop binding becomes a `_binding` temp
// and the object destructure statements are unshifted into the body.
func TestForOfObjectPatternInitializer(t *testing.T) {
	want := `local objs = { {
	x = 1,
}, {
	x = 2,
} }
local sum = 0
for _, _binding in objs do
	local x = _binding.x
	sum += x
end
print(sum)
`
	if got := renderForOfFile(t, "src/objpattern.ts"); got != want {
		t.Errorf("rendered output differs from rbxtsc:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

// TestForOfPatternDefaults: defaults in the loop-header pattern emit the
// extract-then-nil-check shape inside the body, for both array and object
// patterns.
func TestForOfPatternDefaults(t *testing.T) {
	want := `local entries = { { 1, nil }, { nil, 4 } }
local total = 0
for _, _binding in entries do
	local a = _binding[1]
	if a == nil then
		a = 10
	end
	local b = _binding[2]
	if b == nil then
		b = 20
	end
	total += a + b
end
local objs = { {
	x = 1,
}, {} }
for _, _binding in objs do
	local x = _binding.x
	if x == nil then
		x = 5
	end
	total += x
end
print(total)
`
	if got := renderForOfFile(t, "src/defaults.ts"); got != want {
		t.Errorf("rendered output differs from rbxtsc:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

// TestForOfExpressionPrereqs: prereqs of the iterable expression (`i++`
// inside the call argument) land BEFORE the loop; the call itself stays
// inline in the for header.
func TestForOfExpressionPrereqs(t *testing.T) {
	want := `local function make(n)
	return { n, n + 1 }
end
local i = 0
local sum = 0
local _original = i
i += 1
for _, v in make(_original) do
	sum += v
end
print(sum, i)
`
	if got := renderForOfFile(t, "src/prereqs.ts"); got != want {
		t.Errorf("rendered output differs from rbxtsc:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

// TestNestedForOf: the inner loop's unnamed discard temp dedupes against the
// outer one (`_`, then `_1`).
func TestNestedForOf(t *testing.T) {
	want := `local grid = { { 1, 2 }, { 3, 4 } }
local sum = 0
for _, row in grid do
	for _1, cell in row do
		sum += cell
	end
end
print(sum)
`
	if got := renderForOfFile(t, "src/nested.ts"); got != want {
		t.Errorf("rendered output differs from rbxtsc:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

// TestForOfContinue: `continue` passes straight through the statement list —
// for-of has no loop finalizers (unlike for-in over keys in older targets),
// so no finalizer interplay exists.
func TestForOfContinue(t *testing.T) {
	want := `local nums = { 1, 2, 3, 4 }
local odds = 0
for _, n in nums do
	if n % 2 == 0 then
		continue
	end
	odds += n
end
print(odds)
`
	if got := renderForOfFile(t, "src/continue.ts"); got != want {
		t.Errorf("rendered output differs from rbxtsc:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

// TestForOfNoMacroUnionDiagnostic: a union iterable that is not definitely
// any single builder type hits upstream's `type.isUnion()` arm. rbxtsc 3.0.0
// reports exactly "Macro cannot be applied to a union type!".
func TestForOfNoMacroUnionDiagnostic(t *testing.T) {
	ds := forOfDiagnostics(t, "src/union.ts")
	if !hasDiagnostic(ds, "noMacroUnion", "Macro cannot be applied to a union type!") {
		t.Errorf("no noMacroUnion diagnostic; got: %v", ds)
	}
}

// TestForOfNoIterableIterationDiagnostic: iterating a bare Iterable<T> is
// upstream's own error. rbxtsc 3.0.0 reports exactly "Iterating on
// Iterable<T> is not supported! You must use a more specific type.".
func TestForOfNoIterableIterationDiagnostic(t *testing.T) {
	ds := forOfDiagnostics(t, "src/iterable.ts")
	if !hasDiagnostic(ds, "noIterableIteration", "Iterating on Iterable<T> is not supported!") {
		t.Errorf("no noIterableIteration diagnostic; got: %v", ds)
	}
}

// TestForOfRangeMacroDiagnostic: `for (const i of $range(1, 3))` must hit the
// findRangeMacro check BEFORE the expression is transformed — exactly ONE
// rotorNotYetSupported diagnostic. If the order were wrong, the generic call
// macro stand-in would fire first AND $range's Iterable<number> return type
// would then raise noIterableIteration from the builder dispatch.
func TestForOfRangeMacroDiagnostic(t *testing.T) {
	ds := forOfDiagnostics(t, "src/range.ts")
	if len(ds) != 1 {
		t.Fatalf("expected exactly 1 diagnostic, got %d: %v", len(ds), ds)
	}
	if !hasDiagnostic(ds, "rotorNotYetSupported", "macro `$range`") {
		t.Errorf("no rotorNotYetSupported diagnostic for $range; got: %v", ds)
	}
}
