package lex

import "testing"

func TestShortStrings(t *testing.T) {
	ok := func(src string) {
		toks, diags := Tokenize(src)
		if len(diags) != 0 {
			t.Fatalf("%q diags: %v", src, diags)
		}
		if toks[0].Kind != String || toks[0].Text != src {
			t.Fatalf("%q -> %v %q", src, toks[0].Kind, toks[0].Text)
		}
	}
	ok(`"hello"`)
	ok(`'world'`)
	ok(`"a\"b"`)         // escaped quote
	ok(`"tab\there"`)    // escape
	ok(`"\u{1F600}"`)    // unicode escape (bytes skipped, not decoded)
	ok("\"a\\z\n   b\"") // \z line-continuation skips following whitespace incl newline
	ok("'line\\\ncont'") // backslash-newline continuation

	toks, diags := Tokenize(`"oops`)
	if len(diags) != 1 || toks[0].Kind != String {
		t.Fatalf("unterminated: diags=%v kind=%v", diags, toks[0].Kind)
	}
}

func TestLongStringsAndComments(t *testing.T) {
	kindText := func(src string) (Kind, string) {
		toks, diags := Tokenize(src)
		if len(diags) != 0 {
			t.Fatalf("%q diags: %v", src, diags)
		}
		return toks[0].Kind, toks[0].Text
	}
	if k, txt := kindText("[[hello]]"); k != String || txt != "[[hello]]" {
		t.Fatalf("long string -> %v %q", k, txt)
	}
	if k, txt := kindText("[==[a]]b]==]"); k != String || txt != "[==[a]]b]==]" {
		t.Fatalf("leveled long string -> %v %q", k, txt)
	}
	if k, txt := kindText("-- line\nx"); k != Comment || txt != "-- line" {
		t.Fatalf("line comment -> %v %q", k, txt)
	}
	if k, txt := kindText("--[[ block ]] x"); k != Comment || txt != "--[[ block ]]" {
		t.Fatalf("block comment -> %v %q", k, txt)
	}
	if k, txt := kindText("--[==[ b ]==]"); k != Comment || txt != "--[==[ b ]==]" {
		t.Fatalf("leveled block comment -> %v %q", k, txt)
	}
	toks, _ := Tokenize("a[1]")
	if toks[1].Kind != Symbol || toks[1].Text != "[" {
		t.Fatalf("plain bracket -> %v %q", toks[1].Kind, toks[1].Text)
	}
}
