package cst

import (
	"testing"

	"rotor/internal/luau/lex"
)

func TestAttachAndFlatten(t *testing.T) {
	src := "local x = 1 -- hi\nprint(x)\n"
	refs := AttachTrivia(src)
	if Flatten(refs) != src {
		t.Fatalf("flatten mismatch")
	}
	// the trailing comment attaches to the token ending the line ('1'); the
	// newline-bearing whitespace begins the next significant token's leading.
	var firstNum *TokenRef
	for i := range refs {
		if refs[i].Token.Text == "1" {
			firstNum = &refs[i]
		}
	}
	if firstNum == nil {
		t.Fatal("missing '1' token")
	}
	var hasTrailingComment bool
	for _, tr := range firstNum.Trailing {
		if tr.Text == "-- hi" {
			hasTrailingComment = true
		}
	}
	if !hasTrailingComment {
		t.Fatalf("comment not attached as trailing of '1': %+v", firstNum.Trailing)
	}
	if refs[len(refs)-1].Token.Kind != lex.EOF {
		t.Fatalf("last ref not EOF")
	}
}

func TestUnparseLeaf(t *testing.T) {
	refs := AttachTrivia("nil")
	leaf := &Nil{Tok: refs[0]}
	if Unparse(leaf) != "nil" {
		t.Fatalf("unparse leaf = %q", Unparse(leaf))
	}
}
