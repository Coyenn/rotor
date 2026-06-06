package luau

import "testing"

func TestIsValidIdentifier(t *testing.T) {
	valid := []string{"x", "_foo", "A1_b2", "_"}
	invalid := []string{"and", "end", "nil", "local", "until", "then", "elseif", "repeat", "1x", "a-b", "a b", "", "héllo"}
	for _, s := range valid {
		if !IsValidIdentifier(s) {
			t.Errorf("%q should be valid", s)
		}
	}
	for _, s := range invalid {
		if IsValidIdentifier(s) {
			t.Errorf("%q should be invalid", s)
		}
	}
}

func TestIsValidNumberLiteral(t *testing.T) {
	valid := []string{"1", "1_000", "1.5", ".5", "1e5", "1E+5", "1.2e-3", "0b1010", "0B_1010", "0xFF", "0X_ff_0"}
	invalid := []string{"", "abc", "0b", "0x", "1.2.3", "e5"}
	for _, s := range valid {
		if !IsValidNumberLiteral(s) {
			t.Errorf("%q should be valid", s)
		}
	}
	for _, s := range invalid {
		if IsValidNumberLiteral(s) {
			t.Errorf("%q should be invalid", s)
		}
	}
}

func TestMetamethodsAndReserved(t *testing.T) {
	if !IsMetamethod("__index") || IsMetamethod("__banana") {
		t.Error("IsMetamethod")
	}
	if !IsReservedClassField("new") || !IsReservedClassField("__index") || IsReservedClassField("foo") {
		t.Error("IsReservedClassField")
	}
	if !IsReservedIdentifier("TS") || !IsReservedIdentifier("game") || IsReservedIdentifier("myVar") {
		t.Error("IsReservedIdentifier")
	}
}
