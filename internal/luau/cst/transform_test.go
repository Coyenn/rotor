package cst

import "testing"

func TestUnparseWithIdentity(t *testing.T) {
	src := "local x = require(\"./foo\")\nreturn x\n"
	file, _ := Parse(src)
	if got := UnparseWith(file, nil); got != src {
		t.Fatalf("identity UnparseWith = %q", got)
	}
}

func TestUnparseWithReplace(t *testing.T) {
	src := "local x = require(\"./foo\")\n"
	file, _ := Parse(src)
	var call *Call
	EachNode(file, func(n Node) {
		if c, ok := n.(*Call); ok {
			if name, ok := c.Base.(*Name); ok && name.Tok.Token.Text == "require" {
				call = c
			}
		}
	})
	if call == nil {
		t.Fatal("did not find the require call")
	}
	got := UnparseWith(file, map[Node]string{call: "__B.load_3()"})
	want := "local x = __B.load_3()\n"
	if got != want {
		t.Fatalf("UnparseWith = %q, want %q", got, want)
	}
}

func TestStringValue(t *testing.T) {
	cases := map[string]string{
		`"./foo"`:      "./foo",
		`'../bar/baz'`: "../bar/baz",
		`"a\\b"`:       `a\b`,
		"[[raw/path]]": "raw/path",
		"[==[lvl]==]":  "lvl",
	}
	for src, want := range cases {
		file, _ := Parse("return " + src + "\n")
		var s *String
		EachNode(file, func(n Node) {
			if v, ok := n.(*String); ok {
				s = v
			}
		})
		if s == nil {
			t.Fatalf("%q: no string node", src)
		}
		got, ok := StringValue(s)
		if !ok || got != want {
			t.Fatalf("StringValue(%q) = %q,%v want %q", src, got, ok, want)
		}
	}
}
