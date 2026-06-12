package cst

import "rotor/internal/luau/lex"

// Parse parses a full Luau source file into a *File CST and returns any diagnostics.
// Unparse(file) reproduces src byte-for-byte when there are no diagnostics.
func Parse(src string) (*File, []Diagnostic) {
	c := newCursor(src)
	body := c.parseBlock()
	// Consume any leftover tokens (e.g. a stray top-level `end`/`until`/`else`, or
	// junk after a parse error). Each stray token records a diagnostic and is kept in
	// the tree so Unparse stays byte-exact even on invalid input.
	for !c.atEnd() {
		c.errHere("unexpected '" + c.peek().Token.Text + "'")
		body.Stmts = append(body.Stmts, &ErrorStmt{Tok: c.next()})
		rest := c.parseBlock()
		body.Stmts = append(body.Stmts, rest.Stmts...)
	}
	eof := c.next() // the EOF ref (carries trailing end-of-file trivia)
	return &File{Body: body, EOF: eof}, c.diags
}

// atBlockEnd reports whether the cursor is at a token that terminates a block.
func (c *cursor) atBlockEnd() bool {
	return c.atKeyword("end") || c.atKeyword("else") ||
		c.atKeyword("elseif") || c.atKeyword("until")
}

func (c *cursor) parseBlock() *Block {
	b := &Block{}
	for !c.atEnd() && !c.atBlockEnd() {
		if c.atSymbol(";") {
			b.Stmts = append(b.Stmts, &SemiStmt{Tok: c.next()})
			continue
		}
		stmt, isLast := c.parseStatement()
		b.Stmts = append(b.Stmts, stmt)
		if isLast {
			break
		}
	}
	return b
}

func compoundAssignOp(t lex.Token) bool {
	if t.Kind != lex.Symbol {
		return false
	}
	switch t.Text {
	case "+=", "-=", "*=", "/=", "//=", "%=", "^=", "..=":
		return true
	}
	return false
}

// endsStatement reports whether the current token ends/closes a statement (so a
// contextual keyword like `continue` is the statement rather than an identifier).
func (c *cursor) endsStatement() bool {
	return c.atEnd() || c.atSymbol(";") || c.atBlockEnd()
}

// parseStatement parses one statement and reports whether it is a last-statement
// (return/break/continue) that terminates its block.
func (c *cursor) parseStatement() (Node, bool) {
	switch {
	case c.atKeyword("local"):
		return c.parseLocal(), false
	case c.atKeyword("do"):
		return c.parseDo(), false
	case c.atKeyword("while"):
		return c.parseWhile(), false
	case c.atKeyword("repeat"):
		return c.parseRepeat(), false
	case c.atKeyword("for"):
		return c.parseFor(), false
	case c.atKeyword("if"):
		return c.parseIfStmt(), false
	case c.atKeyword("function"):
		return c.parseFuncStmt(), false
	case c.atKeyword("return"):
		return c.parseReturn(), true
	case c.atKeyword("break"):
		return c.parseBreak(), true
	case c.atKeyword("continue") && c.peek2Ends():
		return c.parseContinue(), true
	case c.atKeyword("export") && c.peek2().Token.Kind == lex.Name && c.peek2().Token.Text == "type":
		return c.parseTypeAlias(), false
	case c.atKeyword("type") && c.peek2().Token.Kind == lex.Name:
		return c.parseTypeAlias(), false
	default:
		return c.parseExprStatement(), false
	}
}

// peek2Ends reports whether the token after the current one ends the statement
// (used to disambiguate `continue` the keyword from `continue` the identifier).
func (c *cursor) peek2Ends() bool {
	t := c.peek2().Token
	if t.Kind == lex.EOF || (t.Kind == lex.Symbol && t.Text == ";") {
		return true
	}
	if t.Kind == lex.Name {
		switch t.Text {
		case "end", "else", "elseif", "until":
			return true
		}
	}
	return false
}

func (c *cursor) parseExprStatement() Node {
	first := c.parsePrefixExpr()
	if c.atSymbol("=") || compoundAssignOp(c.peek().Token) || c.atSymbol(",") {
		targets := ExprList{Items: []Node{first}}
		for c.atSymbol(",") {
			targets.Seps = append(targets.Seps, c.next())
			targets.Items = append(targets.Items, c.parsePrefixExpr())
		}
		op := c.next() // = or a compound-assign operator
		var values ExprList
		c.parseExprListInto(&values, "")
		return &AssignStmt{Targets: targets, Op: op, Values: values}
	}
	return &CallStmt{Call: first}
}

func (c *cursor) parseLocal() Node {
	local := c.next()
	if c.atKeyword("function") {
		fn := c.next()
		name := c.expectName()
		return &LocalFuncStmt{Local: local, Func: fn, Name: name, Body: c.parseFuncBody()}
	}
	n := &LocalStmt{Local: local}
	n.Names = append(n.Names, c.parseBinding(true))
	for c.atSymbol(",") {
		n.NameSeps = append(n.NameSeps, c.next())
		n.Names = append(n.Names, c.parseBinding(true))
	}
	if c.atSymbol("=") {
		eq := c.next()
		n.Eq = &eq
		c.parseExprListInto(&n.Values, "")
	}
	return n
}

// parseBinding parses NAME [<attr>] [: Type]. Attributes are only valid on locals.
func (c *cursor) parseBinding(allowAttrib bool) Binding {
	b := Binding{Name: c.expectName()}
	if allowAttrib && c.atSymbol("<") {
		lt := c.next()
		name := c.expectName()
		gt := c.expectSymbol(">")
		b.Attr = &Attrib{Lt: lt, Name: name, Gt: gt}
	}
	if c.atSymbol(":") {
		colon := c.next()
		b.Colon = &colon
		b.Type = c.parseType()
	}
	return b
}

func (c *cursor) parseDo() Node {
	do := c.next()
	body := c.parseBlock()
	end := c.expectKeyword("end")
	return &DoStmt{Do: do, Body: body, End: end}
}

func (c *cursor) parseWhile() Node {
	w := c.next()
	cond := c.parseExpr()
	do := c.expectKeyword("do")
	body := c.parseBlock()
	end := c.expectKeyword("end")
	return &WhileStmt{While: w, Cond: cond, Do: do, Body: body, End: end}
}

func (c *cursor) parseRepeat() Node {
	r := c.next()
	body := c.parseBlock()
	until := c.expectKeyword("until")
	cond := c.parseExpr()
	return &RepeatStmt{Repeat: r, Body: body, Until: until, Cond: cond}
}

func (c *cursor) parseFor() Node {
	forTok := c.next()
	first := c.parseBinding(false)
	if c.atSymbol("=") {
		eq := c.next()
		start := c.parseExpr()
		comma1 := c.expectSymbol(",")
		limit := c.parseExpr()
		n := &NumericForStmt{For: forTok, Var: first, Eq: eq, Start: start, Comma1: comma1, Limit: limit}
		if c.atSymbol(",") {
			comma2 := c.next()
			n.Comma2 = &comma2
			n.Step = c.parseExpr()
		}
		n.Do = c.expectKeyword("do")
		n.Body = c.parseBlock()
		n.End = c.expectKeyword("end")
		return n
	}
	n := &GenericForStmt{For: forTok}
	n.Names = append(n.Names, first)
	for c.atSymbol(",") {
		n.NameSeps = append(n.NameSeps, c.next())
		n.Names = append(n.Names, c.parseBinding(false))
	}
	n.In = c.expectKeyword("in")
	c.parseExprListInto(&n.Exprs, "")
	n.Do = c.expectKeyword("do")
	n.Body = c.parseBlock()
	n.End = c.expectKeyword("end")
	return n
}

func (c *cursor) parseIfStmt() Node {
	n := &IfStmt{If: c.next()}
	n.Cond = c.parseExpr()
	n.Then = c.expectKeyword("then")
	n.Body = c.parseBlock()
	for c.atKeyword("elseif") {
		var cl ElseifClause
		cl.Elseif = c.next()
		cl.Cond = c.parseExpr()
		cl.Then = c.expectKeyword("then")
		cl.Body = c.parseBlock()
		n.Elseifs = append(n.Elseifs, cl)
	}
	if c.atKeyword("else") {
		e := c.next()
		n.Else = &e
		n.ElseBody = c.parseBlock()
	}
	n.End = c.expectKeyword("end")
	return n
}

func (c *cursor) parseFuncStmt() Node {
	fn := c.next()
	name := FuncName{Base: c.expectName()}
	for c.atSymbol(".") {
		dot := c.next()
		nm := c.expectName()
		name.Dots = append(name.Dots, dotName{Dot: dot, Name: nm})
	}
	if c.atSymbol(":") {
		colon := c.next()
		name.Colon = &colon
		m := c.expectName()
		name.Method = &m
	}
	return &FuncStmt{Func: fn, Name: name, Body: c.parseFuncBody()}
}

func (c *cursor) parseFuncBody() *FuncBody {
	fb := &FuncBody{}
	if c.atSymbol("<") {
		fb.Generics = c.parseGenericParams()
	}
	fb.LParen = c.expectSymbol("(")
	if !c.atSymbol(")") && !c.atEnd() {
		fb.Params = append(fb.Params, c.parseParam())
		for c.atSymbol(",") {
			fb.ParamSeps = append(fb.ParamSeps, c.next())
			fb.Params = append(fb.Params, c.parseParam())
		}
	}
	fb.RParen = c.expectSymbol(")")
	if c.atSymbol(":") {
		colon := c.next()
		fb.Colon = &colon
		fb.RetType = c.parseReturnType()
	}
	fb.Body = c.parseBlock()
	fb.End = c.expectKeyword("end")
	return fb
}

func (c *cursor) parseParam() Param {
	if c.atSymbol("...") {
		v := c.next()
		p := Param{Vararg: &v}
		if c.atSymbol(":") {
			colon := c.next()
			p.Colon = &colon
			p.Type = c.parseType()
		}
		return p
	}
	name := c.expectName()
	p := Param{Name: &name}
	if c.atSymbol(":") {
		colon := c.next()
		p.Colon = &colon
		p.Type = c.parseType()
	}
	return p
}

func (c *cursor) parseReturn() Node {
	n := &ReturnStmt{Return: c.next()}
	if !c.endsStatement() {
		c.parseExprListInto(&n.Values, "")
	}
	if c.atSymbol(";") {
		s := c.next()
		n.Semi = &s
	}
	return n
}

func (c *cursor) parseBreak() Node {
	n := &BreakStmt{Break: c.next()}
	if c.atSymbol(";") {
		s := c.next()
		n.Semi = &s
	}
	return n
}

func (c *cursor) parseContinue() Node {
	n := &ContinueStmt{Continue: c.next()}
	if c.atSymbol(";") {
		s := c.next()
		n.Semi = &s
	}
	return n
}

func (c *cursor) parseTypeAlias() Node {
	n := &TypeAliasStmt{}
	if c.atKeyword("export") {
		e := c.next()
		n.Export = &e
	}
	n.Type = c.next() // the "type" keyword
	n.Name = c.expectName()
	if c.atSymbol("<") {
		n.Generics = c.parseGenericParams()
	}
	n.Eq = c.expectSymbol("=")
	n.Value = c.parseType()
	return n
}

func (c *cursor) parseGenericParams() *GenericParams {
	g := &GenericParams{Lt: c.expectSymbol("<")}
	for !c.atSymbol(">") && !c.atEnd() {
		var p GenericParam
		p.Name = c.expectName()
		if c.atSymbol("...") {
			pack := c.next()
			p.Pack = &pack
		}
		if c.atSymbol("=") {
			eq := c.next()
			p.Eq = &eq
			p.Default = c.parseType()
		}
		g.Params = append(g.Params, p)
		if c.atSymbol(",") {
			g.Seps = append(g.Seps, c.next())
		} else {
			break
		}
	}
	g.Gt = c.expectSymbol(">")
	return g
}
