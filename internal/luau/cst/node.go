package cst

import "strings"

// Node is any CST node. tokens yields, in source order, every TokenRef and child
// Node beneath this node (used by Unparse and traversal). A node yields a non-nil
// ref for each of its own leaf tokens and a non-nil child for each child subtree.
type Node interface {
	tokens(yield func(ref *TokenRef, child Node))
}

// Unparse reproduces the exact source text spanned by n.
func Unparse(n Node) string {
	var b strings.Builder
	var walk func(Node)
	walk = func(n Node) {
		n.tokens(func(ref *TokenRef, child Node) {
			switch {
			case ref != nil:
				b.WriteString(ref.text())
			case child != nil:
				walk(child)
			}
		})
	}
	walk(n)
	return b.String()
}

// Nil is the simplest leaf node, used to bootstrap Unparse before the grammar lands.
type Nil struct {
	Tok TokenRef
}

func (n *Nil) tokens(yield func(ref *TokenRef, child Node)) { yield(&n.Tok, nil) }
