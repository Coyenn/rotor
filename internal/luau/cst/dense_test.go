package cst

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"rotor/internal/luau/lex"
)

// significantTokens returns the kind+text of every non-trivia token, optionally
// dropping ';' separators (which are semantically inert).
func significantTokens(src string, dropSemi bool) []lex.Token {
	toks, _ := lex.Tokenize(src)
	var out []lex.Token
	for _, t := range toks {
		if t.Kind == lex.Whitespace || t.Kind == lex.Comment || t.Kind == lex.EOF {
			continue
		}
		if dropSemi && t.Kind == lex.Symbol && t.Text == ";" {
			continue
		}
		out = append(out, t)
	}
	return out
}

func denseSrc(src string) string {
	file, _ := Parse(src)
	return Dense(file)
}

func TestDenseUnits(t *testing.T) {
	cases := map[string]string{
		"local x = 1\n":           "local x=1",
		"local  y  =  a  -  -b\n": "local y=a- -b",
		"local x = 1\n(f)()\n":    "local x=1;(f)()",
		"local t = { a = 1 }\n":   "local t={a=1}",
		"return a and b or c\n":   "return a and b or c",
		"x = 1 -- comment\n":      "x=1",
		"local s = `a {b} c`\n":   "local s=`a {b} c`",
	}
	for src, want := range cases {
		if got := denseSrc(src); got != want {
			t.Errorf("Dense(%q) = %q, want %q", src, got, want)
		}
	}
}

// TestDenseCorpus is the dense generator's correctness gate: minifying any corpus
// file must produce valid Luau whose significant tokens (ignoring inserted ';') are
// exactly the original's — i.e. nothing is merged, dropped, or reordered.
func TestDenseCorpus(t *testing.T) {
	root := os.Getenv("ROTOR_LUAU_CORPUS")
	if root == "" {
		t.Skip("set ROTOR_LUAU_CORPUS to a directory of .luau/.lua files")
	}
	var origBytes, denseBytes, count int
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if !strings.HasSuffix(path, ".luau") && !strings.HasSuffix(path, ".lua") {
			return nil
		}
		count++
		data, _ := os.ReadFile(path)
		src := string(data)
		dense := denseSrc(src)

		// dense output must itself parse cleanly
		if _, diags := Parse(dense); len(diags) != 0 {
			t.Errorf("%s: dense output has %d diag(s), first: %q", path, len(diags), diags[0].Message)
			return nil
		}
		// every significant token (ignoring ';') must be preserved exactly
		a := significantTokens(src, true)
		b := significantTokens(dense, true)
		if len(a) != len(b) {
			t.Errorf("%s: token count %d -> %d", path, len(a), len(b))
			return nil
		}
		for i := range a {
			if a[i].Kind != b[i].Kind || a[i].Text != b[i].Text {
				t.Errorf("%s: token %d %q -> %q", path, i, a[i].Text, b[i].Text)
				return nil
			}
		}
		origBytes += len(src)
		denseBytes += len(dense)
		return nil
	})
	if origBytes > 0 {
		t.Logf("dense-minified %d files: %d -> %d bytes (%.1f%% of original)",
			count, origBytes, denseBytes, 100*float64(denseBytes)/float64(origBytes))
	}
}
