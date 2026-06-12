package cst

// Statement nodes and the shared building blocks (bindings, function bodies, names,
// generic parameters). As with expressions, every tokens()/emit() yields TokenRefs
// and children in exact source order so Unparse is byte-exact.

// File is the top-level node: a block plus the EOF token, which carries any trailing
// end-of-file trivia (whitespace/comments). Unparse(File) reproduces the whole source.
type File struct {
	Body *Block
	EOF  TokenRef
}

func (f *File) tokens(y func(*TokenRef, Node)) {
	y(nil, f.Body)
	y(&f.EOF, nil)
}

// Block is a statement list.
type Block struct {
	Stmts []Node
}

func (b *Block) tokens(y func(*TokenRef, Node)) {
	for i := range b.Stmts {
		y(nil, b.Stmts[i])
	}
}

// ---- shared building blocks ----

// Attrib is a Luau attribute on a local binding: <const>, <close>.
type Attrib struct {
	Lt   TokenRef
	Name TokenRef
	Gt   TokenRef
}

func (a *Attrib) emit(y func(*TokenRef, Node)) {
	y(&a.Lt, nil)
	y(&a.Name, nil)
	y(&a.Gt, nil)
}

// Binding is a name with an optional attribute and an optional ": Type" annotation.
// Used for local names, parameters, and for-loop variables.
type Binding struct {
	Name  TokenRef
	Attr  *Attrib
	Colon *TokenRef
	Type  Node
}

func (b *Binding) emit(y func(*TokenRef, Node)) {
	y(&b.Name, nil)
	if b.Attr != nil {
		b.Attr.emit(y)
	}
	if b.Colon != nil {
		y(b.Colon, nil)
		y(nil, b.Type)
	}
}

// Param is a function parameter: a named binding or a vararg (... with optional type).
type Param struct {
	Vararg *TokenRef
	Name   *TokenRef
	Colon  *TokenRef
	Type   Node
}

func (p *Param) emit(y func(*TokenRef, Node)) {
	if p.Vararg != nil {
		y(p.Vararg, nil)
	} else {
		y(p.Name, nil)
	}
	if p.Colon != nil {
		y(p.Colon, nil)
		y(nil, p.Type)
	}
}

// dotName is a ".name" segment.
type dotName struct {
	Dot  TokenRef
	Name TokenRef
}

// GenericParam is one entry of a generic parameter list: T, T..., or T = Default.
type GenericParam struct {
	Name    TokenRef
	Pack    *TokenRef // "..."
	Eq      *TokenRef
	Default Node
}

type GenericParams struct {
	Lt     TokenRef
	Params []GenericParam
	Seps   []TokenRef
	Gt     TokenRef
}

func (g *GenericParams) emit(y func(*TokenRef, Node)) {
	y(&g.Lt, nil)
	for i := range g.Params {
		p := &g.Params[i]
		y(&p.Name, nil)
		if p.Pack != nil {
			y(p.Pack, nil)
		}
		if p.Eq != nil {
			y(p.Eq, nil)
			y(nil, p.Default)
		}
		if i < len(g.Seps) {
			y(&g.Seps[i], nil)
		}
	}
	y(&g.Gt, nil)
}

// FuncBody is the shared "(params) [: ret] block end" of function expressions and
// declarations (optionally preceded by generic parameters).
type FuncBody struct {
	Generics  *GenericParams
	LParen    TokenRef
	Params    []Param
	ParamSeps []TokenRef
	RParen    TokenRef
	Colon     *TokenRef
	RetType   Node
	Body      *Block
	End       TokenRef
}

func (b *FuncBody) emit(y func(*TokenRef, Node)) {
	if b.Generics != nil {
		b.Generics.emit(y)
	}
	y(&b.LParen, nil)
	for i := range b.Params {
		b.Params[i].emit(y)
		if i < len(b.ParamSeps) {
			y(&b.ParamSeps[i], nil)
		}
	}
	y(&b.RParen, nil)
	if b.Colon != nil {
		y(b.Colon, nil)
		y(nil, b.RetType)
	}
	y(nil, b.Body)
	y(&b.End, nil)
}

// FuncExpr is an anonymous function expression: function(...) ... end.
type FuncExpr struct {
	Func TokenRef
	Body *FuncBody
}

func (n *FuncExpr) tokens(y func(*TokenRef, Node)) {
	y(&n.Func, nil)
	n.Body.emit(y)
}

// FuncName is the dotted/method name of a function declaration: a.b.c:d.
type FuncName struct {
	Base   TokenRef
	Dots   []dotName
	Colon  *TokenRef
	Method *TokenRef
}

func (f *FuncName) emit(y func(*TokenRef, Node)) {
	y(&f.Base, nil)
	for i := range f.Dots {
		y(&f.Dots[i].Dot, nil)
		y(&f.Dots[i].Name, nil)
	}
	if f.Colon != nil {
		y(f.Colon, nil)
		y(f.Method, nil)
	}
}

// ---- statements ----

type SemiStmt struct{ Tok TokenRef }

func (n *SemiStmt) tokens(y func(*TokenRef, Node)) { y(&n.Tok, nil) }

// ErrorStmt wraps a single token that could not be parsed as part of a statement
// (recovery), keeping it in the tree so Unparse remains byte-exact on invalid input.
type ErrorStmt struct{ Tok TokenRef }

func (n *ErrorStmt) tokens(y func(*TokenRef, Node)) { y(&n.Tok, nil) }

type LocalStmt struct {
	Local    TokenRef
	Names    []Binding
	NameSeps []TokenRef
	Eq       *TokenRef
	Values   ExprList
}

func (n *LocalStmt) tokens(y func(*TokenRef, Node)) {
	y(&n.Local, nil)
	for i := range n.Names {
		n.Names[i].emit(y)
		if i < len(n.NameSeps) {
			y(&n.NameSeps[i], nil)
		}
	}
	if n.Eq != nil {
		y(n.Eq, nil)
		n.Values.tokens(y)
	}
}

type LocalFuncStmt struct {
	Local TokenRef
	Func  TokenRef
	Name  TokenRef
	Body  *FuncBody
}

func (n *LocalFuncStmt) tokens(y func(*TokenRef, Node)) {
	y(&n.Local, nil)
	y(&n.Func, nil)
	y(&n.Name, nil)
	n.Body.emit(y)
}

type AssignStmt struct {
	Targets ExprList
	Op      TokenRef
	Values  ExprList
}

func (n *AssignStmt) tokens(y func(*TokenRef, Node)) {
	n.Targets.tokens(y)
	y(&n.Op, nil)
	n.Values.tokens(y)
}

type CallStmt struct{ Call Node }

func (n *CallStmt) tokens(y func(*TokenRef, Node)) { y(nil, n.Call) }

type DoStmt struct {
	Do   TokenRef
	Body *Block
	End  TokenRef
}

func (n *DoStmt) tokens(y func(*TokenRef, Node)) {
	y(&n.Do, nil)
	y(nil, n.Body)
	y(&n.End, nil)
}

type WhileStmt struct {
	While TokenRef
	Cond  Node
	Do    TokenRef
	Body  *Block
	End   TokenRef
}

func (n *WhileStmt) tokens(y func(*TokenRef, Node)) {
	y(&n.While, nil)
	y(nil, n.Cond)
	y(&n.Do, nil)
	y(nil, n.Body)
	y(&n.End, nil)
}

type RepeatStmt struct {
	Repeat TokenRef
	Body   *Block
	Until  TokenRef
	Cond   Node
}

func (n *RepeatStmt) tokens(y func(*TokenRef, Node)) {
	y(&n.Repeat, nil)
	y(nil, n.Body)
	y(&n.Until, nil)
	y(nil, n.Cond)
}

type NumericForStmt struct {
	For    TokenRef
	Var    Binding
	Eq     TokenRef
	Start  Node
	Comma1 TokenRef
	Limit  Node
	Comma2 *TokenRef
	Step   Node
	Do     TokenRef
	Body   *Block
	End    TokenRef
}

func (n *NumericForStmt) tokens(y func(*TokenRef, Node)) {
	y(&n.For, nil)
	n.Var.emit(y)
	y(&n.Eq, nil)
	y(nil, n.Start)
	y(&n.Comma1, nil)
	y(nil, n.Limit)
	if n.Comma2 != nil {
		y(n.Comma2, nil)
		y(nil, n.Step)
	}
	y(&n.Do, nil)
	y(nil, n.Body)
	y(&n.End, nil)
}

type GenericForStmt struct {
	For      TokenRef
	Names    []Binding
	NameSeps []TokenRef
	In       TokenRef
	Exprs    ExprList
	Do       TokenRef
	Body     *Block
	End      TokenRef
}

func (n *GenericForStmt) tokens(y func(*TokenRef, Node)) {
	y(&n.For, nil)
	for i := range n.Names {
		n.Names[i].emit(y)
		if i < len(n.NameSeps) {
			y(&n.NameSeps[i], nil)
		}
	}
	y(&n.In, nil)
	n.Exprs.tokens(y)
	y(&n.Do, nil)
	y(nil, n.Body)
	y(&n.End, nil)
}

type ElseifClause struct {
	Elseif TokenRef
	Cond   Node
	Then   TokenRef
	Body   *Block
}

type IfStmt struct {
	If       TokenRef
	Cond     Node
	Then     TokenRef
	Body     *Block
	Elseifs  []ElseifClause
	Else     *TokenRef
	ElseBody *Block
	End      TokenRef
}

func (n *IfStmt) tokens(y func(*TokenRef, Node)) {
	y(&n.If, nil)
	y(nil, n.Cond)
	y(&n.Then, nil)
	y(nil, n.Body)
	for i := range n.Elseifs {
		c := &n.Elseifs[i]
		y(&c.Elseif, nil)
		y(nil, c.Cond)
		y(&c.Then, nil)
		y(nil, c.Body)
	}
	if n.Else != nil {
		y(n.Else, nil)
		y(nil, n.ElseBody)
	}
	y(&n.End, nil)
}

type FuncStmt struct {
	Func TokenRef
	Name FuncName
	Body *FuncBody
}

func (n *FuncStmt) tokens(y func(*TokenRef, Node)) {
	y(&n.Func, nil)
	n.Name.emit(y)
	n.Body.emit(y)
}

type ReturnStmt struct {
	Return TokenRef
	Values ExprList
	Semi   *TokenRef
}

func (n *ReturnStmt) tokens(y func(*TokenRef, Node)) {
	y(&n.Return, nil)
	n.Values.tokens(y)
	if n.Semi != nil {
		y(n.Semi, nil)
	}
}

type BreakStmt struct {
	Break TokenRef
	Semi  *TokenRef
}

func (n *BreakStmt) tokens(y func(*TokenRef, Node)) {
	y(&n.Break, nil)
	if n.Semi != nil {
		y(n.Semi, nil)
	}
}

type ContinueStmt struct {
	Continue TokenRef
	Semi     *TokenRef
}

func (n *ContinueStmt) tokens(y func(*TokenRef, Node)) {
	y(&n.Continue, nil)
	if n.Semi != nil {
		y(n.Semi, nil)
	}
}

type TypeAliasStmt struct {
	Export   *TokenRef
	Type     TokenRef
	Name     TokenRef
	Generics *GenericParams
	Eq       TokenRef
	Value    Node
}

func (n *TypeAliasStmt) tokens(y func(*TokenRef, Node)) {
	if n.Export != nil {
		y(n.Export, nil)
	}
	y(&n.Type, nil)
	y(&n.Name, nil)
	if n.Generics != nil {
		n.Generics.emit(y)
	}
	y(&n.Eq, nil)
	y(nil, n.Value)
}
