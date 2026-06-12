package cst

import "testing"

func TestCursorBasics(t *testing.T) {
	c := newCursor("a + b")
	if c.peek().Token.Text != "a" {
		t.Fatalf("peek = %q", c.peek().Token.Text)
	}
	a := c.next()
	if a.Token.Text != "a" || c.peek().Token.Text != "+" {
		t.Fatalf("after next: %q peek %q", a.Token.Text, c.peek().Token.Text)
	}
	if !c.atSymbol("+") {
		t.Fatalf("expected + symbol")
	}
	if c.atEnd() {
		t.Fatalf("not at end yet")
	}
}
