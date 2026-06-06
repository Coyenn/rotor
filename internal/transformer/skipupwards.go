package transformer

import "rotor/tsgo/ast"

// isUpwardsSkipKind matches the wrapper kinds skipped by upstream
// util/traversal.ts skipUpwards/skipDownwards: NonNullExpression,
// ParenthesizedExpression, AsExpression, TypeAssertionExpression,
// SatisfiesExpression.
func isUpwardsSkipKind(kind ast.Kind) bool {
	switch kind {
	case ast.KindNonNullExpression,
		ast.KindParenthesizedExpression,
		ast.KindAsExpression,
		ast.KindTypeAssertionExpression,
		ast.KindSatisfiesExpression:
		return true
	}
	return false
}

// SkipUpwards ports util/traversal.ts skipUpwards: climbs through enclosing
// NonNullExpression / ParenthesizedExpression / AsExpression /
// TypeAssertionExpression / SatisfiesExpression wrappers so `(x as Foo)!`
// queries the outermost wrapper.
func SkipUpwards(node *ast.Node) *ast.Node {
	parent := node.Parent
	for parent != nil && isUpwardsSkipKind(parent.Kind) {
		node = parent
		parent = node.Parent
	}
	return node
}

// SkipDownwards ports util/traversal.ts skipDownwards: the inverse — unwraps
// the same wrapper kinds by following `.expression`, so `(x as Foo)!`
// resolves to `x`.
func SkipDownwards(node *ast.Node) *ast.Node {
	for isUpwardsSkipKind(node.Kind) {
		node = node.Expression()
	}
	return node
}
