package transformer

import (
	"testing"

	"rotor/internal/luau"
)

// TestArrayMacroTableInventory pins the table keys against the upstream
// PROPERTY_CALL_MACROS inventory (propertyCallMacros.ts L163-585, L587-734):
// READONLY_ARRAY_METHODS has exactly 15 entries and ARRAY_METHODS exactly 9.
// A typo'd key would otherwise only surface as a MacroManager registration
// audit failure ("could not find method") in checker-full tests.
func TestArrayMacroTableInventory(t *testing.T) {
	wantReadonly := []string{
		"isEmpty", "join", "move", "includes", "indexOf", "every", "some",
		"forEach", "map", "mapFiltered", "filterUndefined", "filter",
		"reduce", "find", "findIndex",
	}
	wantArray := []string{
		"push", "pop", "shift", "unshift", "insert", "remove",
		"unorderedRemove", "sort", "clear",
	}

	check := func(name string, table map[string]PropertyCallMacro, want []string) {
		if len(table) != len(want) {
			t.Errorf("%s has %d entries, want %d", name, len(table), len(want))
		}
		for _, key := range want {
			if table[key] == nil {
				t.Errorf("%s[%q] missing or nil", name, key)
			}
		}
	}
	check("READONLY_ARRAY_METHODS", readonlyArrayMethods, wantReadonly)
	check("ARRAY_METHODS", arrayMethods, wantArray)
}

// TestArrayMacroOffsets proves the ±1 index adjustments for the pure-
// expression macros (no checker, no call node needed): the TS 0-based index
// arguments gain +1, and indexOf's 1-based table.find result loses 1 through
// the raw `or 0` binary (rendered with the precedence parens).
func TestArrayMacroOffsets(t *testing.T) {
	s := NewTestState()
	arr := func() luau.Expression { return luau.ID("arr") }
	tests := []struct {
		table  map[string]PropertyCallMacro
		method string
		args   []luau.Expression
		want   string
	}{
		{readonlyArrayMethods, "isEmpty", nil, "#arr == 0"},
		{readonlyArrayMethods, "move", []luau.Expression{luau.Num(0), luau.Num(2), luau.Num(1)}, "table.move(arr, 1, 3, 2)"},
		{readonlyArrayMethods, "move", []luau.Expression{luau.Num(0), luau.Num(2), luau.Num(1), luau.ID("dst")}, "table.move(arr, 1, 3, 2, dst)"},
		{readonlyArrayMethods, "includes", []luau.Expression{luau.Num(7)}, "table.find(arr, 7) ~= nil"},
		{readonlyArrayMethods, "includes", []luau.Expression{luau.Num(7), luau.ID("from")}, "table.find(arr, 7, from + 1) ~= nil"},
		{readonlyArrayMethods, "indexOf", []luau.Expression{luau.Num(7)}, "(table.find(arr, 7) or 0) - 1"},
		{readonlyArrayMethods, "indexOf", []luau.Expression{luau.Num(7), luau.Num(1)}, "(table.find(arr, 7, 2) or 0) - 1"},
		{arrayMethods, "shift", nil, "table.remove(arr, 1)"},
		{arrayMethods, "insert", []luau.Expression{luau.Num(0), luau.Str("x")}, `table.insert(arr, 1, "x")`},
		{arrayMethods, "insert", []luau.Expression{luau.ID("i"), luau.Str("x")}, `table.insert(arr, i + 1, "x")`},
		{arrayMethods, "insert", []luau.Expression{luau.NewBinary(luau.ID("i"), "-", luau.Num(1)), luau.Str("x")}, `table.insert(arr, i, "x")`},
		{arrayMethods, "remove", []luau.Expression{luau.Num(4)}, "table.remove(arr, 5)"},
	}
	for _, tt := range tests {
		macro := tt.table[tt.method]
		expr, prereqs := s.Capture(func() luau.Expression {
			return macro(s, nil, arr(), tt.args)
		})
		if prereqs.IsNonEmpty() {
			t.Errorf("%s: pure-expression macro must not produce prereqs, got %d", tt.method, prereqs.Size())
		}
		if got := renderMacroExpr(t, expr); got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.method, got, tt.want)
		}
	}
}
