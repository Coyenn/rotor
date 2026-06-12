package lex

import (
	"strings"
	"testing"
)

func FuzzRoundtrip(f *testing.F) {
	seeds := []string{
		"local x = 1", "`a{b}c`", "[==[x]==]", "-- c\n", "0x1.8p2", "'\\z\n  q'",
		"a..=b//c", "}{}{", "```", "[[", "--[[", "\"\\", "`{`{`x`}`}`",
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, src string) {
		toks, _ := Tokenize(src) // must never panic
		var b strings.Builder
		for _, tk := range toks {
			b.WriteString(tk.Text)
		}
		if b.String() != src {
			t.Fatalf("roundtrip mismatch (len %d vs %d)", b.Len(), len(src))
		}
		for _, tk := range toks {
			if tk.Start.Offset < 0 || tk.End.Offset > len(src) || tk.Start.Offset > tk.End.Offset {
				t.Fatalf("bad span %v..%v for %v", tk.Start, tk.End, tk.Kind)
			}
		}
	})
}
