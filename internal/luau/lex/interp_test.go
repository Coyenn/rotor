package lex

import "testing"

func TestInterpolatedStrings(t *testing.T) {
	// no hole
	toks, diags := Tokenize("`hello`")
	if len(diags) != 0 || toks[0].Kind != InterpSimple || toks[0].Text != "`hello`" {
		t.Fatalf("simple -> %v %q diags=%v", toks[0].Kind, toks[0].Text, diags)
	}
	// one hole: `a{x}b`
	toks, _ = Tokenize("`a{x}b`")
	wantKinds := []Kind{InterpStart, Name, InterpEnd, EOF}
	gk := tokKinds(toks)
	for i := range wantKinds {
		if gk[i] != wantKinds[i] {
			t.Fatalf("`a{x}b` kinds = %v", gk)
		}
	}
	if toks[0].Text != "`a{" || toks[2].Text != "}b`" {
		t.Fatalf("texts = %q %q", toks[0].Text, toks[2].Text)
	}
	// two holes -> InterpMid in the middle
	toks, _ = Tokenize("`{a}{b}`")
	if got := tokKinds(toks); got[0] != InterpStart || got[2] != InterpMid || got[4] != InterpEnd {
		t.Fatalf("`{a}{b}` kinds = %v", got)
	}
	// a table literal inside a hole: braces balance, not an interp boundary
	toks, _ = Tokenize("`{ {x=1} }`")
	got := nonEOFTexts(toks)
	want := []string{"`{", " ", "{", "x", "=", "1", "}", " ", "}`"}
	if len(got) != len(want) {
		t.Fatalf("nested braces -> %q", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("nested braces -> %q, want %q", got, want)
		}
	}
	// nested interpolation: `a{`b`}c`
	toks, _ = Tokenize("`a{`b`}c`")
	gk = tokKinds(toks)
	if gk[0] != InterpStart || gk[1] != InterpSimple || gk[2] != InterpEnd {
		t.Fatalf("nested interp kinds = %v", gk)
	}
}
