package transformer_test

import (
	"testing"
)

// Expected text is byte-for-byte rbxtsc 3.0.0 output — see the note at the
// top of enums_test.go (shared renderModuleFixture harness).

// TestNamespaceBasic pins the namespace emit shapes (digest §5.4, oracle
// §6.3):
//
//   - `local X = {}` + do-block; with value exports the first do-statement is
//     `local _container = X`, and each export-declaring statement is followed
//     by `_container.<name> = <name>` (the statement-list ExportInfo mapping);
//   - `export let` is EXCLUDED from the mapping — SetModuleIDBySymbol routes
//     its declaration to `_container.mut = 2` and namespace-internal
//     reads/writes through `_container.mut` (bump), while outside accesses go
//     through `Outer.mut`;
//   - namespaces with no value exports get a bare do-block (NoExports);
//   - dotted `namespace AB.CD` is a nested ModuleDeclaration: recursion plus
//     an explicit `_container.CD = CD`, container temps numbered by creation
//     order (`_container`, `_container_1`);
//   - `declare namespace` (dispatch declare skip) and type-only namespaces
//     (isInstantiatedModule) emit nothing;
//   - `export namespace` needs nothing special at statement level;
//   - one statement declaring MULTIPLE exports (`export const p = 1, q = 2`)
//     appends both `_container.<name>` assignments after that statement, in
//     binder/source order.
func TestNamespaceBasic(t *testing.T) {
	got, s := renderModuleFixture(t, "namespaces", "src/basic.ts")

	want := `local Outer = {}
do
	local _container = Outer
	local value = 1
	_container.value = value
	_container.mut = 2
	local function fn()
		return value
	end
	_container.fn = fn
	local function bump()
		_container.mut += 1
		return _container.mut
	end
	_container.bump = bump
	local hidden = 3
	local Inner = {}
	do
		local _container_1 = Inner
		local deep = hidden
		_container_1.deep = deep
	end
	_container.Inner = Inner
end
local function useNs()
	print(Outer.value, Outer.fn(), Outer.Inner.deep, Outer.mut, Outer.bump())
	Outer.mut = 5
end
local NoExports = {}
do
	local x = 1
	print(x)
end
local AB = {}
do
	local _container = AB
	local CD = {}
	do
		local _container_1 = CD
		local v = 1
		_container_1.v = v
	end
	_container.CD = CD
end
local function useAB()
	print(AB.CD.v)
end
local ExpNs = {}
do
	local _container = ExpNs
	local e = 1
	_container.e = e
	local p = 1
	local q = 2
	_container.p = p
	_container.q = q
end
`
	if got != want {
		t.Errorf("rendered output differs from rbxtsc:\ngot:\n%s\nwant:\n%s", got, want)
	}
	if ds := s.Diags.Flush(); len(ds) != 0 {
		t.Errorf("unexpected diagnostics: %v", ds)
	}
}

// TestNamespaceHoisted pins the use-before-declare namespace (oracle §6.4):
// the hoist machinery emits `local NS` at the premature use and the namespace
// transform takes the Assignment branch (`NS = {}`), do-block unchanged.
func TestNamespaceHoisted(t *testing.T) {
	got, s := renderModuleFixture(t, "namespaces", "src/hoist.ts")

	want := `local NS
local function earlyNs()
	return NS.v
end
NS = {}
do
	local _container = NS
	local v = 7
	_container.v = v
end
`
	if got != want {
		t.Errorf("rendered output differs from rbxtsc:\ngot:\n%s\nwant:\n%s", got, want)
	}
	if ds := s.Diags.Flush(); len(ds) != 0 {
		t.Errorf("unexpected diagnostics: %v", ds)
	}
}

// TestNamespaceWithEnum pins the enum-inside-namespace interplay (oracle
// §6.4): the enum's do-block statements run inside the namespace do-block and
// the ExportInfo mapping appends `_container.Inner2 = Inner2` AFTER the
// enum's statements (the enum statement is the mapping key).
func TestNamespaceWithEnum(t *testing.T) {
	got, s := renderModuleFixture(t, "namespaces", "src/hasenum.ts")

	want := `local HasEnum = {}
do
	local _container = HasEnum
	local Inner2
	do
		local _inverse = {}
		Inner2 = setmetatable({}, {
			__index = _inverse,
		})
		Inner2.X = 0
		_inverse[0] = "X"
	end
	_container.Inner2 = Inner2
	local c = Inner2.X
	_container.c = c
end
local function useHasEnum()
	print(HasEnum.c)
end
`
	if got != want {
		t.Errorf("rendered output differs from rbxtsc:\ngot:\n%s\nwant:\n%s", got, want)
	}
	if ds := s.Diags.Flush(); len(ds) != 0 {
		t.Errorf("unexpected diagnostics: %v", ds)
	}
}

// TestNamespaceMerging pins the merging ban (digest §5.2/§7): merged
// namespace declarations emit nothing and report noNamespaceMerging once per
// symbol. namespace+namespace AND namespace+function-with-body both count
// toward isDeclarationOfNamespace's filter (the function itself still emits).
func TestNamespaceMerging(t *testing.T) {
	got, s := renderModuleFixture(t, "namespaces", "src/merge.ts")

	want := `local function f()
end
`
	if got != want {
		t.Errorf("rendered output differs:\ngot:\n%s\nwant:\n%s", got, want)
	}
	ds := s.Diags.Flush()
	if len(ds) != 2 {
		t.Fatalf("expected exactly 2 diagnostics (one per merged symbol), got %d: %v", len(ds), ds)
	}
	for _, d := range ds {
		if d.Code != "noNamespaceMerging" {
			t.Errorf("expected noNamespaceMerging, got %q", d.Code)
		}
		if d.Message != "Namespace merging is not supported!" {
			t.Errorf("unexpected message: %q", d.Message)
		}
	}
}
