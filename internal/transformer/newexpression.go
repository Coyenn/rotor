package transformer

import (
	"rotor/internal/luau"
	"rotor/tsgo/ast"
)

// transformNewExpression ports
// nodes/expressions/transformNewExpression.ts (COMPLETE):
// construct-signature-symbol lookup -> constructor macro dispatch -> fallback
// `X.new(args)`. The fallback covers user classes AND `new Instance("Part")`
// -> `Instance.new("Part")` — there is NO separate Instance macro upstream.
// `new C` without parens -> empty args.
func transformNewExpression(s *State, node *ast.Node) luau.Expression {
	newExpr := node.AsNewExpression()
	validateNotAnyType(s, newExpr.Expression)

	if symbol := getFirstConstructSymbol(s, newExpr.Expression); symbol != nil {
		if macro := s.Macros().GetConstructorMacro(symbol); macro != nil {
			return macro(s, node)
		}
	}

	expression := convertToIndexableExpression(TransformExpression(s, newExpr.Expression))
	var args []luau.Expression
	if newExpr.Arguments != nil {
		args = ensureTransformOrder(s, newExpr.Arguments.Nodes)
	}
	return luau.NewCall(luau.NewPropertyAccess(expression, "new"), luau.NewList(args...))
}
