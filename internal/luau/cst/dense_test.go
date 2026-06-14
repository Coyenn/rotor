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

// TestDenseNumberAdjacencySeparator guards the number-glue hazard: rotor's
// lexer splits `100print` into two tokens, but Luau reads it as one malformed
// number, so the dense writer must keep a separator after a numeric literal when
// the next token would glue onto it.
func TestDenseNumberAdjacencySeparator(t *testing.T) {
	cases := map[string]string{
		"local x = 100\nprint(x)\n":  "local x=100 print(x)",
		"return 1 .. 2\n":            "return 1 ..2",
		"local a = 0xFF\nreturn a\n": "local a=0xFF return a",
		"for i = 1, 10 do end\n":     "for i=1,10 do end", // ',' separates safely
	}
	for src, want := range cases {
		t.Run(src, func(t *testing.T) {
			got := denseSrc(src)
			if got != want {
				t.Errorf("Dense(%q) = %q, want %q", src, got, want)
			}
			if _, diags := Parse(got); len(diags) != 0 {
				t.Errorf("dense output %q does not re-parse: %v", got, diags)
			}
		})
	}
}

// TestConvertIndexToField covers the string-index-to-field minifier rewrite:
// valid identifiers collapse to dotted access (in both indexing and table keys),
// keywords and non-identifiers stay bracketed, and the faithful Dense leaves
// everything untouched.
func TestConvertIndexToField(t *testing.T) {
	convert := func(src string) string {
		file, _ := Parse(src)
		return DenseWith(file, DenseOptions{ConvertIndexToField: true})
	}
	cases := map[string]string{
		`return a["foo"]`:         `return a.foo`,
		`return a["foo"]["bar"]`:  `return a.foo.bar`,
		`a["x"] = 1`:              `a.x=1`,
		`return a["foo bar"]`:     `return a["foo bar"]`, // space: not an identifier
		`return a["then"]`:        `return a["then"]`,    // reserved keyword
		`return a["1x"]`:          `return a["1x"]`,      // not an identifier
		`return a[b]`:             `return a[b]`,         // non-string key
		"local t = {['foo'] = 1}": `local t={foo=1}`,
		"local t = {['end'] = 1}": `local t={['end']=1}`, // keyword key kept verbatim
		"local t = {['a b'] = 1}": `local t={['a b']=1}`, // non-ident key kept verbatim
	}
	for src, want := range cases {
		t.Run(src, func(t *testing.T) {
			got := convert(src)
			if got != want {
				t.Errorf("convert(%q) = %q, want %q", src, got, want)
			}
			// The rewrite must always produce parseable Luau.
			if _, diags := Parse(got); len(diags) != 0 {
				t.Errorf("convert(%q) = %q does not re-parse: %v", src, got, diags)
			}
			// The faithful Dense must NOT convert (significant tokens preserved).
			file, _ := Parse(src)
			if faithful := Dense(file); strings.Contains(faithful, ".foo") && !strings.Contains(src, ".foo") {
				t.Errorf("faithful Dense(%q) = %q unexpectedly converted", src, faithful)
			}
		})
	}
}

// TestConvertIndexToFieldCorpusStaysValid proves the rewrite never produces
// invalid Luau across the whole corpus (the conversion is opt-in, so the
// faithful TestDenseCorpus token-equivalence gate is unaffected).
func TestConvertIndexToFieldCorpusStaysValid(t *testing.T) {
	root := os.Getenv("ROTOR_LUAU_CORPUS")
	if root == "" {
		t.Skip("set ROTOR_LUAU_CORPUS to a directory of .luau/.lua files")
	}
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if !strings.HasSuffix(path, ".luau") && !strings.HasSuffix(path, ".lua") {
			return nil
		}
		data, _ := os.ReadFile(path)
		out, diags := MinifyWith(string(data), MinifyOptions{ConvertIndexToField: true})
		if len(diags) != 0 {
			return nil // a file that doesn't parse cleanly isn't our concern here
		}
		if _, d2 := Parse(out); len(d2) != 0 {
			t.Errorf("%s: index-to-field minify output does not re-parse: %q", path, d2[0].Message)
		}
		return nil
	})
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
