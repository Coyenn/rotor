package transformer_test

import (
	"path/filepath"
	"testing"

	"rotor/internal/luau/render"
	"rotor/internal/transformer"
)

// The expected text below is byte-for-byte what rbxtsc 3.0.0 emits for the
// same source (verified via a scratch file compiled through
// testdata/diff/project — see tools/oracle/oracle.ps1 for the technique;
// header and trailing `return nil` stripped, those belong to
// TransformSourceFile).

// TestNewExpressionAgainstRbxtsc pins every constructor macro and the
// `X.new(args)` fallback:
//
//   - `new Array<T>()`            -> `{}`; with args -> `table.create(...)`
//     (second arg fills)
//   - `new Set([...])` spread-free TS array literal -> set literal
//     (`[v] = true` fields), including prereq-bearing elements
//     (ensureTransformOrder pins `i++` into `_original`)
//   - `new Set(expr)` non-literal arg -> `local _set = {}` +
//     `for _, _v in expr do _set[_v] = true end` loop path
//   - `new Map([[k, v], ...])` whose TRANSFORMED arg is a luau array of
//     arrays -> map literal; `new Map([])` -> `{}` (vacuously all-arrays)
//   - `new Map(expr)` (identifier or call) -> loop path
//     `for _, _v in expr do _map[_v[1]] = _v[2] end` — the decision is on
//     the transformed luau AST, so `makePairs()` falls to the loop even
//     though its TS type is an array of pairs
//   - `new WeakMap()` / `new WeakSet()` -> `setmetatable({}, { __mode = "k" })`
//     (weak KEYS for both, upstream quirk)
//   - `new ReadonlyMap/ReadonlySet(...)` -> plain Map/Set macros
//   - `new Instance("Part")` -> `Instance.new("Part")` (fallback; upstream
//     has no Instance macro)
//   - user interface with a construct signature (`declare const Thing:
//     ThingConstructor`) -> `Thing.new(5)` (fallback)
func TestNewExpressionAgainstRbxtsc(t *testing.T) {
	s := buildState(t, filepath.Join("testdata", "new"), "src/constructors.ts")

	statements := transformer.TransformStatementList(s, s.SourceFile.AsNode(), s.SourceFile.Statements.Nodes, nil)
	if ds := s.Diags.Flush(); len(ds) != 0 {
		t.Fatalf("unexpected diagnostics: %v", ds)
	}

	want := `local a = {}
local b = table.create(8)
local c = table.create(4, "x")
local s1 = {
	["a"] = true,
	["b"] = true,
}
local s2 = {}
local src = { "x", "y" }
local _set = {}
for _, _v in src do
	_set[_v] = true
end
local s3 = _set
local i = 0
local _original = i
i += 1
local s4 = {
	[_original] = true,
	[i] = true,
}
local m1 = {
	a = 1,
	b = 2,
}
local m2 = {}
local entries = { { "k", 3 } }
local _map = {}
for _, _v in entries do
	_map[_v[1]] = _v[2]
end
local m3 = _map
local m4 = {}
local function makePairs()
	return { { "p", 9 } }
end
local _map_1 = {}
for _, _v in makePairs() do
	_map_1[_v[1]] = _v[2]
end
local m5 = _map_1
local wm = setmetatable({}, {
	__mode = "k",
})
local ws = setmetatable({}, {
	__mode = "k",
})
local rm = {
	a = 1,
}
local rs = {
	["q"] = true,
}
local part = Instance.new("Part")
local t = Thing.new(5)
print(a, b, c, s1, s2, s3, s4, m1, m2, m3, m4, m5, wm, ws, rm, rs, part, t)
`
	if got := render.RenderAST(statements); got != want {
		t.Errorf("rendered output differs from rbxtsc:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

// TestConstructorMacroIdentifierWithoutNew: a constructor-macro identifier
// indexed without `new` (`const A = Array`) keeps raising loudly through the
// centralized MacroManager (upstream raises noConstructorMacroWithoutNew from
// transformIdentifier; rotor's compiler-types sentinel covers it until the
// Phase 3b misuse guards land).
func TestConstructorMacroIdentifierWithoutNew(t *testing.T) {
	s := buildState(t, filepath.Join("testdata", "new"), "src/macroident.ts")
	transformer.TransformStatementList(s, s.SourceFile.AsNode(), s.SourceFile.Statements.Nodes, nil)
	if ds := s.Diags.Flush(); !hasDiagnostic(ds, "rotorNotYetSupported", "macro `Array`") {
		t.Errorf("no rotorNotYetSupported diagnostic for `Array` without new; got: %v", ds)
	}
}

// TestNewPromiseRaisesIdentifierMacroDiagnostic: `new Promise(...)` has a
// construct symbol but NO constructor macro (upstream PromiseConstructor is
// not in CONSTRUCTOR_MACROS) — the fallback transforms the `Promise`
// identifier, which is the registered-but-unimplemented identifier macro
// (upstream: state.TS(node, "Promise")) and must raise the sentinel, never
// emit a bare `Promise.new(...)`.
func TestNewPromiseRaisesIdentifierMacroDiagnostic(t *testing.T) {
	s := buildState(t, filepath.Join("testdata", "new"), "src/promise.ts")
	transformer.TransformStatementList(s, s.SourceFile.AsNode(), s.SourceFile.Statements.Nodes, nil)
	if ds := s.Diags.Flush(); !hasDiagnostic(ds, "rotorNotYetSupported", "macro `Promise`") {
		t.Errorf("no rotorNotYetSupported diagnostic for `new Promise`; got: %v", ds)
	}
}
