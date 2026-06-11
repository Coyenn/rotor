package transformer

import (
	"rotor/internal/luau"
	"rotor/tsgo/ast"
)

// transformVoidExpression ports expressions/transformVoidExpression.ts: the
// operand runs for its side effects in statement form (prereq'd), and the
// expression itself evaluates to nil.
func transformVoidExpression(s *State, node *ast.Node) luau.Expression {
	s.PrereqList(transformExpressionStatementInner(s, SkipDownwards(node.AsVoidExpression().Expression)))
	return luau.Nil()
}
