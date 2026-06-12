package cst

// This file defines the concrete CST node structs and their source-order tokens()
// visitors. The cardinal rule: tokens() must yield every TokenRef and child Node in
// exact source order, so Unparse reproduces the source byte-for-byte.

// ---- shared list helper ----

// ExprList is a comma-separated list of expressions. Seps holds the separators;
// len(Seps) == len(Items)-1 normally, or == len(Items) when a trailing separator is
// present (e.g. table fields). Items may be empty.
type ExprList struct {
	Items []Node
	Seps  []TokenRef
}

func (l *ExprList) tokens(yield func(ref *TokenRef, child Node)) {
	for i := range l.Items {
		yield(nil, l.Items[i])
		if i < len(l.Seps) {
			yield(&l.Seps[i], nil)
		}
	}
}

// ---- leaf literals ----

type True struct{ Tok TokenRef }
type False struct{ Tok TokenRef }
type Vararg struct{ Tok TokenRef }
type Number struct{ Tok TokenRef }
type String struct{ Tok TokenRef }
type Name struct{ Tok TokenRef }

func (n *True) tokens(y func(*TokenRef, Node))   { y(&n.Tok, nil) }
func (n *False) tokens(y func(*TokenRef, Node))  { y(&n.Tok, nil) }
func (n *Vararg) tokens(y func(*TokenRef, Node)) { y(&n.Tok, nil) }
func (n *Number) tokens(y func(*TokenRef, Node)) { y(&n.Tok, nil) }
func (n *String) tokens(y func(*TokenRef, Node)) { y(&n.Tok, nil) }
func (n *Name) tokens(y func(*TokenRef, Node))   { y(&n.Tok, nil) }

// InterpString is a backtick interpolated string. Simple is set for the hole-free
// form `...`. Otherwise Start is the opening `...{ chunk, and each hole i is Exprs[i]
// followed by Mids[i] (an InterpMid }...{ chunk, the last being the InterpEnd }...`).
type InterpString struct {
	Simple *TokenRef
	Start  *TokenRef
	Exprs  []Node
	Mids   []TokenRef
}

func (n *InterpString) tokens(y func(*TokenRef, Node)) {
	if n.Simple != nil {
		y(n.Simple, nil)
		return
	}
	y(n.Start, nil)
	for i := range n.Exprs {
		y(nil, n.Exprs[i])
		y(&n.Mids[i], nil)
	}
}

// ---- parenthesized ----

type Paren struct {
	Open  TokenRef
	Inner Node
	Close TokenRef
}

func (n *Paren) tokens(y func(*TokenRef, Node)) {
	y(&n.Open, nil)
	y(nil, n.Inner)
	y(&n.Close, nil)
}

// ---- suffix chains (postfix on a base expression) ----

// Index is a dotted field access: base.name
type Index struct {
	Base Node
	Dot  TokenRef
	Name TokenRef
}

func (n *Index) tokens(y func(*TokenRef, Node)) {
	y(nil, n.Base)
	y(&n.Dot, nil)
	y(&n.Name, nil)
}

// IndexExpr is a computed index: base[key]
type IndexExpr struct {
	Base  Node
	Open  TokenRef
	Key   Node
	Close TokenRef
}

func (n *IndexExpr) tokens(y func(*TokenRef, Node)) {
	y(nil, n.Base)
	y(&n.Open, nil)
	y(nil, n.Key)
	y(&n.Close, nil)
}

// Call is base(args), base"str", or base{table}. Args is one of *ParenArgs, *String,
// *Table.
type Call struct {
	Base Node
	Args Node
}

func (n *Call) tokens(y func(*TokenRef, Node)) {
	y(nil, n.Base)
	y(nil, n.Args)
}

// MethodCall is base:name(args)
type MethodCall struct {
	Base  Node
	Colon TokenRef
	Name  TokenRef
	Args  Node
}

func (n *MethodCall) tokens(y func(*TokenRef, Node)) {
	y(nil, n.Base)
	y(&n.Colon, nil)
	y(&n.Name, nil)
	y(nil, n.Args)
}

// ParenArgs is a parenthesized argument list: ( exprlist )
type ParenArgs struct {
	Open  TokenRef
	List  ExprList
	Close TokenRef
}

func (n *ParenArgs) tokens(y func(*TokenRef, Node)) {
	y(&n.Open, nil)
	n.List.tokens(y)
	y(&n.Close, nil)
}

// ---- operators ----

type Unary struct {
	Op      TokenRef
	Operand Node
}

func (n *Unary) tokens(y func(*TokenRef, Node)) {
	y(&n.Op, nil)
	y(nil, n.Operand)
}

type Binary struct {
	Left  Node
	Op    TokenRef
	Right Node
}

func (n *Binary) tokens(y func(*TokenRef, Node)) {
	y(nil, n.Left)
	y(&n.Op, nil)
	y(nil, n.Right)
}

// ---- if-then-else expression ----

type IfExprClause struct {
	Elseif TokenRef
	Cond   Node
	Then   TokenRef
	Value  Node
}

type IfExpr struct {
	If       TokenRef
	Cond     Node
	Then     TokenRef
	ThenExpr Node
	Elseifs  []IfExprClause
	Else     TokenRef
	ElseExpr Node
}

func (n *IfExpr) tokens(y func(*TokenRef, Node)) {
	y(&n.If, nil)
	y(nil, n.Cond)
	y(&n.Then, nil)
	y(nil, n.ThenExpr)
	for i := range n.Elseifs {
		c := &n.Elseifs[i]
		y(&c.Elseif, nil)
		y(nil, c.Cond)
		y(&c.Then, nil)
		y(nil, c.Value)
	}
	y(&n.Else, nil)
	y(nil, n.ElseExpr)
}

// ---- table constructor ----

// TableField is one field of a table constructor. Exactly one shape is populated:
//   - keyed:      [Key] = Value   (LBracket, Key, RBracket, Eq, Value set)
//   - named:      Name  = Value   (NameTok, Eq, Value set)
//   - positional: Value           (only Value set)
//
// Sep is the trailing ',' or ';' if present.
type TableField struct {
	LBracket *TokenRef
	Key      Node
	RBracket *TokenRef
	NameTok  *TokenRef
	Eq       *TokenRef
	Value    Node
	Sep      *TokenRef
}

func (f *TableField) tokens(y func(*TokenRef, Node)) {
	switch {
	case f.LBracket != nil:
		y(f.LBracket, nil)
		y(nil, f.Key)
		y(f.RBracket, nil)
		y(f.Eq, nil)
		y(nil, f.Value)
	case f.NameTok != nil:
		y(f.NameTok, nil)
		y(f.Eq, nil)
		y(nil, f.Value)
	default:
		y(nil, f.Value)
	}
	if f.Sep != nil {
		y(f.Sep, nil)
	}
}

type Table struct {
	Open   TokenRef
	Fields []TableField
	Close  TokenRef
}

func (n *Table) tokens(y func(*TokenRef, Node)) {
	y(&n.Open, nil)
	for i := range n.Fields {
		n.Fields[i].tokens(y)
	}
	y(&n.Close, nil)
}
