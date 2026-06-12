package cst

import "rotor/internal/luau/lex"

// ---- type nodes ----

// TypeAssert is an expression-level `expr :: Type` assertion.
type TypeAssert struct {
	Expr       Node
	ColonColon TokenRef
	Type       Node
}

func (n *TypeAssert) tokens(y func(*TokenRef, Node)) {
	y(nil, n.Expr)
	y(&n.ColonColon, nil)
	y(nil, n.Type)
}

// TypeName is a named type with optional module path and type arguments: Foo,
// Foo.Bar, Array<number>, Map<K, V>.
type TypeName struct {
	Base TokenRef
	Dots []dotName
	Args *TypeArgs
	Pack *TokenRef // trailing "..." for a generic type pack reference (T...)
}

func (n *TypeName) tokens(y func(*TokenRef, Node)) {
	y(&n.Base, nil)
	for i := range n.Dots {
		y(&n.Dots[i].Dot, nil)
		y(&n.Dots[i].Name, nil)
	}
	if n.Args != nil {
		n.Args.emit(y)
	}
	if n.Pack != nil {
		y(n.Pack, nil)
	}
}

type TypeArgs struct {
	Lt    TokenRef
	Items []Node
	Seps  []TokenRef
	Gt    TokenRef
}

func (a *TypeArgs) emit(y func(*TokenRef, Node)) {
	y(&a.Lt, nil)
	for i := range a.Items {
		y(nil, a.Items[i])
		if i < len(a.Seps) {
			y(&a.Seps[i], nil)
		}
	}
	y(&a.Gt, nil)
}

// SingletonType is a literal type: a string literal, true, false, or nil.
type SingletonType struct{ Tok TokenRef }

func (n *SingletonType) tokens(y func(*TokenRef, Node)) { y(&n.Tok, nil) }

// TypeOptional is T?.
type TypeOptional struct {
	Type     Node
	Question TokenRef
}

func (n *TypeOptional) tokens(y func(*TokenRef, Node)) {
	y(nil, n.Type)
	y(&n.Question, nil)
}

// TypeBinary is a union (A | B) or intersection (A & B).
type TypeBinary struct {
	Left  Node
	Op    TokenRef
	Right Node
}

func (n *TypeBinary) tokens(y func(*TokenRef, Node)) {
	y(nil, n.Left)
	y(&n.Op, nil)
	y(nil, n.Right)
}

// TypeLead is a leading union/intersection bar: | A or & A.
type TypeLead struct {
	Op   TokenRef
	Type Node
}

func (n *TypeLead) tokens(y func(*TokenRef, Node)) {
	y(&n.Op, nil)
	y(nil, n.Type)
}

// VariadicType is ...T (a type pack).
type VariadicType struct {
	Dots TokenRef
	Type Node
}

func (n *VariadicType) tokens(y func(*TokenRef, Node)) {
	y(&n.Dots, nil)
	if n.Type != nil {
		y(nil, n.Type)
	}
}

// TypeofType is typeof(expr).
type TypeofType struct {
	Typeof TokenRef
	LParen TokenRef
	Expr   Node
	RParen TokenRef
}

func (n *TypeofType) tokens(y func(*TokenRef, Node)) {
	y(&n.Typeof, nil)
	y(&n.LParen, nil)
	y(nil, n.Expr)
	y(&n.RParen, nil)
}

// TypeParam is one parameter of a function type: an optional vararg, an optional
// "name:" label, and the type.
type TypeParam struct {
	Vararg *TokenRef
	Name   *TokenRef
	Colon  *TokenRef
	Type   Node
}

func (p *TypeParam) emit(y func(*TokenRef, Node)) {
	if p.Vararg != nil {
		y(p.Vararg, nil)
	}
	if p.Name != nil {
		y(p.Name, nil)
		y(p.Colon, nil)
	}
	if p.Type != nil {
		y(nil, p.Type)
	}
}

// FunctionType is (params) -> ret, optionally generic. A bare parenthesized type
// (no arrow) is represented with Arrow == nil.
type FunctionType struct {
	Generics  *GenericParams
	LParen    TokenRef
	Params    []TypeParam
	ParamSeps []TokenRef
	RParen    TokenRef
	Arrow     *TokenRef
	Ret       Node
}

func (n *FunctionType) tokens(y func(*TokenRef, Node)) {
	if n.Generics != nil {
		n.Generics.emit(y)
	}
	y(&n.LParen, nil)
	for i := range n.Params {
		n.Params[i].emit(y)
		if i < len(n.ParamSeps) {
			y(&n.ParamSeps[i], nil)
		}
	}
	y(&n.RParen, nil)
	if n.Arrow != nil {
		y(n.Arrow, nil)
		y(nil, n.Ret)
	}
}

// TableType is { ... }: an array element type, named properties, and/or an indexer.
type TableTypeField struct {
	LBracket *TokenRef
	Key      Node
	RBracket *TokenRef
	NameTok  *TokenRef
	Colon    *TokenRef
	Value    Node
	Sep      *TokenRef
}

func (f *TableTypeField) emit(y func(*TokenRef, Node)) {
	switch {
	case f.LBracket != nil:
		y(f.LBracket, nil)
		y(nil, f.Key)
		y(f.RBracket, nil)
		y(f.Colon, nil)
		y(nil, f.Value)
	case f.NameTok != nil:
		y(f.NameTok, nil)
		y(f.Colon, nil)
		y(nil, f.Value)
	default:
		y(nil, f.Value)
	}
	if f.Sep != nil {
		y(f.Sep, nil)
	}
}

type TableType struct {
	Open   TokenRef
	Fields []TableTypeField
	Close  TokenRef
}

func (n *TableType) tokens(y func(*TokenRef, Node)) {
	y(&n.Open, nil)
	for i := range n.Fields {
		n.Fields[i].emit(y)
	}
	y(&n.Close, nil)
}

// ---- type parser ----

func (c *cursor) parseReturnType() Node { return c.parseType() }

func (c *cursor) parseType() Node {
	var t Node
	if c.atSymbol("|") || c.atSymbol("&") {
		op := c.next()
		t = &TypeLead{Op: op, Type: c.parseTypePostfix()}
	} else {
		t = c.parseTypePostfix()
	}
	for c.atSymbol("|") || c.atSymbol("&") {
		op := c.next()
		rhs := c.parseTypePostfix()
		t = &TypeBinary{Left: t, Op: op, Right: rhs}
	}
	return t
}

func (c *cursor) parseTypePostfix() Node {
	t := c.parseTypePrimary()
	for c.atSymbol("?") {
		q := c.next()
		t = &TypeOptional{Type: t, Question: q}
	}
	return t
}

func (c *cursor) parseTypePrimary() Node {
	switch {
	case c.atSymbol("("):
		return c.parseParenType()
	case c.atSymbol("<"):
		g := c.parseGenericParams()
		ft := c.parseParenType()
		if f, ok := ft.(*FunctionType); ok {
			f.Generics = g
		}
		return ft
	case c.atSymbol("{"):
		return c.parseTableType()
	case c.atSymbol("..."):
		dots := c.next()
		v := &VariadicType{Dots: dots}
		if !c.atSymbol(")") && !c.atSymbol(",") && !c.atSymbol(">") && !c.atEnd() {
			v.Type = c.parseType()
		}
		return v
	case c.atKeyword("typeof") && c.peek2().Token.Kind == lex.Symbol && c.peek2().Token.Text == "(":
		return c.parseTypeof()
	case c.atKeyword("nil"), c.atKeyword("true"), c.atKeyword("false"):
		return &SingletonType{Tok: c.next()}
	case c.peek().Token.Kind == lex.String:
		return &SingletonType{Tok: c.next()}
	case c.peek().Token.Kind == lex.Name:
		return c.parseTypeName()
	default:
		c.errHere("expected a type")
		return &SingletonType{Tok: c.next()}
	}
}

func (c *cursor) parseTypeName() Node {
	n := &TypeName{Base: c.next()}
	for c.atSymbol(".") {
		dot := c.next()
		nm := c.expectName()
		n.Dots = append(n.Dots, dotName{Dot: dot, Name: nm})
	}
	if c.atSymbol("<") {
		n.Args = c.parseTypeArgs()
	}
	if c.atSymbol("...") {
		pack := c.next()
		n.Pack = &pack
	}
	return n
}

func (c *cursor) parseTypeArgs() *TypeArgs {
	a := &TypeArgs{Lt: c.next()}
	for !c.atSymbol(">") && !c.atEnd() {
		a.Items = append(a.Items, c.parseType())
		if c.atSymbol(",") {
			a.Seps = append(a.Seps, c.next())
		} else {
			break
		}
	}
	a.Gt = c.expectSymbol(">")
	return a
}

func (c *cursor) parseParenType() Node {
	open := c.next() // (
	ft := &FunctionType{LParen: open}
	if !c.atSymbol(")") && !c.atEnd() {
		ft.Params = append(ft.Params, c.parseTypeParam())
		for c.atSymbol(",") {
			ft.ParamSeps = append(ft.ParamSeps, c.next())
			ft.Params = append(ft.Params, c.parseTypeParam())
		}
	}
	ft.RParen = c.expectSymbol(")")
	if c.atSymbol("->") {
		arrow := c.next()
		ft.Arrow = &arrow
		ft.Ret = c.parseReturnType()
	}
	return ft
}

func (c *cursor) parseTypeParam() TypeParam {
	if c.atSymbol("...") {
		dots := c.next()
		tp := TypeParam{Vararg: &dots}
		if !c.atSymbol(")") && !c.atSymbol(",") && !c.atEnd() {
			tp.Type = c.parseType()
		}
		return tp
	}
	if c.peek().Token.Kind == lex.Name && c.peek2().Token.Kind == lex.Symbol && c.peek2().Token.Text == ":" {
		name := c.next()
		colon := c.next()
		return TypeParam{Name: &name, Colon: &colon, Type: c.parseType()}
	}
	return TypeParam{Type: c.parseType()}
}

func (c *cursor) parseTableType() Node {
	n := &TableType{Open: c.expectSymbol("{")}
	for !c.atSymbol("}") && !c.atEnd() {
		f := c.parseTableTypeField()
		if c.atSymbol(",") || c.atSymbol(";") {
			sep := c.next()
			f.Sep = &sep
		}
		n.Fields = append(n.Fields, f)
		if f.Sep == nil {
			break
		}
	}
	n.Close = c.expectSymbol("}")
	return n
}

func (c *cursor) parseTableTypeField() TableTypeField {
	switch {
	case c.atSymbol("["):
		lb := c.next()
		key := c.parseType()
		rb := c.expectSymbol("]")
		colon := c.expectSymbol(":")
		val := c.parseType()
		return TableTypeField{LBracket: &lb, Key: key, RBracket: &rb, Colon: &colon, Value: val}
	case c.peek().Token.Kind == lex.Name && c.peek2().Token.Kind == lex.Symbol && c.peek2().Token.Text == ":":
		name := c.next()
		colon := c.next()
		return TableTypeField{NameTok: &name, Colon: &colon, Value: c.parseType()}
	default:
		return TableTypeField{Value: c.parseType()}
	}
}

func (c *cursor) parseTypeof() Node {
	typeofTok := c.next()
	lp := c.expectSymbol("(")
	e := c.parseExpr()
	rp := c.expectSymbol(")")
	return &TypeofType{Typeof: typeofTok, LParen: lp, Expr: e, RParen: rp}
}
