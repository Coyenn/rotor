package transformer

import (
	"rotor/internal/luau"
	"rotor/tsgo/ast"
)

// transformDeleteExpression ports expressions/transformDeleteExpression.ts:
// the access transforms perform the actual `= nil` emit when they see a
// DeleteExpression parent; this node only preserves prereq ordering and the
// `true` result in value position.
func transformDeleteExpression(s *State, node *ast.Node) luau.Expression {
	TransformExpression(s, node.AsDeleteExpression().Expression)
	if !isUsedAsStatement(node) {
		return luau.Bool(true)
	}
	return luau.NewNone()
}
