package cst

import "rotor/internal/luau/lex"

type cursor struct {
	refs  []TokenRef
	pos   int
	diags []Diagnostic
}

// Diagnostic is a parser/lexer diagnostic with a source position.
type Diagnostic struct {
	Pos     lex.Pos
	Message string
}

func newCursor(src string) *cursor {
	toks, ldiags := lex.Tokenize(src)
	c := &cursor{refs: attachTrivia(toks)}
	for _, d := range ldiags {
		c.diags = append(c.diags, Diagnostic{Pos: d.Pos, Message: d.Message})
	}
	return c
}

// peek returns the current ref without consuming it. At/after EOF it returns the EOF
// ref (the cursor never indexes out of range).
func (c *cursor) peek() *TokenRef {
	if c.pos >= len(c.refs) {
		return &c.refs[len(c.refs)-1]
	}
	return &c.refs[c.pos]
}

// peek2 returns the ref after the current one (clamped at the EOF ref). Used for
// one-token lookahead (e.g. distinguishing `name = value` table fields).
func (c *cursor) peek2() *TokenRef {
	if i := c.pos + 1; i < len(c.refs) {
		return &c.refs[i]
	}
	return &c.refs[len(c.refs)-1]
}

// next consumes and returns the current ref (clamped at the EOF ref).
func (c *cursor) next() TokenRef {
	r := *c.peek()
	if c.pos < len(c.refs)-1 {
		c.pos++
	}
	return r
}

func (c *cursor) atEnd() bool { return c.peek().Token.Kind == lex.EOF }

func (c *cursor) atSymbol(s string) bool {
	t := c.peek().Token
	return t.Kind == lex.Symbol && t.Text == s
}

// atKeyword reports whether the current ref is a Name token with the given keyword
// text (Luau keywords are lexed as Name; the parser classifies them).
func (c *cursor) atKeyword(kw string) bool {
	t := c.peek().Token
	return t.Kind == lex.Name && t.Text == kw
}

func (c *cursor) errHere(msg string) {
	c.diags = append(c.diags, Diagnostic{Pos: c.peek().Token.Start, Message: msg})
}

// expectSymbol consumes the current ref if it is the given symbol; otherwise records
// a diagnostic and returns the current ref as best-effort recovery.
func (c *cursor) expectSymbol(s string) TokenRef {
	if c.atSymbol(s) {
		return c.next()
	}
	c.errHere("expected '" + s + "'")
	return c.next()
}

func (c *cursor) expectKeyword(kw string) TokenRef {
	if c.atKeyword(kw) {
		return c.next()
	}
	c.errHere("expected '" + kw + "'")
	return c.next()
}

// expectName consumes the current ref if it is a Name token; otherwise records a
// diagnostic and returns the current ref as best-effort recovery.
func (c *cursor) expectName() TokenRef {
	if c.peek().Token.Kind == lex.Name {
		return c.next()
	}
	c.errHere("expected a name")
	return c.next()
}
