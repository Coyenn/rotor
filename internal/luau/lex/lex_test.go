package lex

import "testing"

func tokKinds(toks []Token) []Kind {
	ks := make([]Kind, len(toks))
	for i, t := range toks {
		ks[i] = t.Kind
	}
	return ks
}

func nonEOFTexts(toks []Token) []string {
	var out []string
	for _, t := range toks {
		if t.Kind == EOF {
			break
		}
		out = append(out, t.Text)
	}
	return out
}

func TestTokenizeEmpty(t *testing.T) {
	toks, diags := Tokenize("")
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	if len(toks) != 1 || toks[0].Kind != EOF {
		t.Fatalf("want [EOF], got %v", toks)
	}
	if toks[0].Start != (Pos{0, 1, 1}) {
		t.Fatalf("EOF pos = %v", toks[0].Start)
	}
}

func TestWhitespaceAndNames(t *testing.T) {
	toks, diags := Tokenize("  local  x")
	if len(diags) != 0 {
		t.Fatalf("diags: %v", diags)
	}
	want := []Kind{Whitespace, Name, Whitespace, Name, EOF}
	got := tokKinds(toks)
	if len(got) != len(want) {
		t.Fatalf("kinds = %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("kinds = %v, want %v", got, want)
		}
	}
	if toks[1].Text != "local" || toks[3].Text != "x" {
		t.Fatalf("texts = %q %q", toks[1].Text, toks[3].Text)
	}
	toks2, _ := Tokenize("a\nb")
	if toks2[2].Start != (Pos{2, 2, 1}) {
		t.Fatalf("second name start = %v", toks2[2].Start)
	}
}

func TestSymbols(t *testing.T) {
	cases := map[string][]string{
		"a + b":      {"a", " ", "+", " ", "b"},
		"x...y":      {"x", "...", "y"},
		"a..=b":      {"a", "..=", "b"},
		"a//=b":      {"a", "//=", "b"},
		"1//2":       {"1", "//", "2"},
		"a<=b>=c~=d": {"a", "<=", "b", ">=", "c", "~=", "d"},
		"a::T->b":    {"a", "::", "T", "->", "b"},
		"f().x":      {"f", "(", ")", ".", "x"},
	}
	for src, wantTexts := range cases {
		toks, _ := Tokenize(src)
		got := nonEOFTexts(toks)
		if len(got) != len(wantTexts) {
			t.Fatalf("%q -> %q, want %q", src, got, wantTexts)
		}
		for i := range wantTexts {
			if got[i] != wantTexts[i] {
				t.Fatalf("%q -> %q, want %q", src, got, wantTexts)
			}
		}
	}
}

func TestDiagnosticsRecover(t *testing.T) {
	toks, diags := Tokenize("a $ b")
	if len(diags) != 1 {
		t.Fatalf("want 1 diag, got %v", diags)
	}
	last := toks[len(toks)-1]
	if last.Kind != EOF {
		t.Fatalf("want trailing EOF")
	}
	var sawB bool
	for _, tk := range toks {
		if tk.Kind == Name && tk.Text == "b" {
			sawB = true
		}
	}
	if !sawB {
		t.Fatalf("recovery dropped tokens: %v", tokKinds(toks))
	}
	if diags[0].Pos.Col == 0 {
		t.Fatalf("diag missing position")
	}
}
