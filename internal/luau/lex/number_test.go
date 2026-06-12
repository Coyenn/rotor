package lex

import "testing"

func TestNumbers(t *testing.T) {
	one := func(src string) {
		toks, diags := Tokenize(src)
		if len(diags) != 0 {
			t.Fatalf("%q diags: %v", src, diags)
		}
		if toks[0].Kind != Number || toks[0].Text != src {
			t.Fatalf("%q -> kind %v text %q", src, toks[0].Kind, toks[0].Text)
		}
	}
	for _, src := range []string{
		"0", "123", "1_000", "1.5", ".5", "3.", "1e10", "1E+5", "1.2e-3",
		"0xFF", "0X_ff_0", "0b1010", "0B_1010", "0x1.8p1", "0x1p-2",
	} {
		one(src)
	}
	// '..' must NOT be swallowed into a number
	toks, _ := Tokenize("1..2")
	if toks[0].Text != "1" || toks[1].Text != ".." || toks[2].Text != "2" {
		t.Fatalf("1..2 -> %q %q %q", toks[0].Text, toks[1].Text, toks[2].Text)
	}
	toks2, _ := Tokenize("1.5.foo")
	if toks2[0].Text != "1.5" || toks2[1].Text != "." || toks2[2].Text != "foo" {
		t.Fatalf("1.5.foo -> %q %q %q", toks2[0].Text, toks2[1].Text, toks2[2].Text)
	}
}
