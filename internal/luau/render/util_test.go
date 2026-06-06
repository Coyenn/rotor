package render

import (
	"testing"

	"rotor/internal/luau"
)

func TestGetSafeBracketEquals(t *testing.T) {
	cases := map[string]string{
		"hello":       "",
		"a ]] b":      "=",
		"a ]=] b":     "",
		"ends with ]": "=",
		"a ]=] ]] b":  "==",
		"x]":          "=",
	}
	for in, want := range cases {
		if got := getSafeBracketEquals(in); got != want {
			t.Errorf("getSafeBracketEquals(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNeedsParentheses(t *testing.T) {
	// (a + b) * c — lower precedence child on the left of higher precedence parent
	inner := luau.NewBinary(luau.ID("a"), "+", luau.ID("b"))
	luau.NewBinary(inner, "*", luau.ID("c"))
	if !needsParentheses(inner) {
		t.Error("(a + b) * c: left + needs parens under *")
	}

	// a + b * c — higher precedence child needs none
	inner2 := luau.NewBinary(luau.ID("b"), "*", luau.ID("c"))
	luau.NewBinary(luau.ID("a"), "+", inner2)
	if needsParentheses(inner2) {
		t.Error("a + b * c: right * needs no parens under +")
	}

	// a - (b - c) — equal precedence on the right needs parens
	inner3 := luau.NewBinary(luau.ID("b"), "-", luau.ID("c"))
	luau.NewBinary(luau.ID("a"), "-", inner3)
	if !needsParentheses(inner3) {
		t.Error("a - (b - c): equal precedence right operand needs parens")
	}

	// a - b - c (left-assoc: left child same precedence, no parens)
	inner4 := luau.NewBinary(luau.ID("a"), "-", luau.ID("b"))
	luau.NewBinary(inner4, "-", luau.ID("c"))
	if needsParentheses(inner4) {
		t.Error("(a - b) - c: left operand needs no parens")
	}
}

func TestRenderStateIndent(t *testing.T) {
	s := NewRenderState()
	if got := s.Line("x"); got != "x\n" {
		t.Errorf("Line at depth 0 = %q", got)
	}
	out := s.Block(func() string { return s.Line("y") })
	if out != "\ty\n" {
		t.Errorf("Line at depth 1 = %q", out)
	}
}
