package render

import "rotor/internal/luau"

// Render dispatches on node type — ports upstream render.ts KIND_TO_RENDERER.
func Render(s *RenderState, node luau.Node) string {
	switch n := node.(type) {
	// indexable expressions
	case *luau.Identifier:
		return renderIdentifier(s, n)
	case *luau.TemporaryIdentifier:
		return renderTemporaryIdentifier(s, n)
	case *luau.ComputedIndexExpression:
		return renderComputedIndexExpression(s, n)
	case *luau.PropertyAccessExpression:
		return renderPropertyAccessExpression(s, n)
	case *luau.CallExpression:
		return renderCallExpression(s, n)
	case *luau.MethodCallExpression:
		return renderMethodCallExpression(s, n)
	case *luau.ParenthesizedExpression:
		return renderParenthesizedExpression(s, n)
	// expressions
	case *luau.None:
		panic("Cannot render None")
	case *luau.NilLiteral:
		return "nil"
	case *luau.FalseLiteral:
		return "false"
	case *luau.TrueLiteral:
		return "true"
	case *luau.NumberLiteral:
		return renderNumberLiteral(s, n)
	case *luau.StringLiteral:
		return renderStringLiteral(s, n)
	case *luau.VarArgsLiteral:
		return "..."
	case *luau.FunctionExpression:
		return renderFunctionExpression(s, n)
	case *luau.BinaryExpression:
		return renderBinaryExpression(s, n)
	case *luau.UnaryExpression:
		return renderUnaryExpression(s, n)
	case *luau.IfExpression:
		return renderIfExpression(s, n)
	case *luau.InterpolatedString:
		return renderInterpolatedString(s, n)
	case *luau.Array:
		return renderArray(s, n)
	case *luau.Map:
		return renderMap(s, n)
	case *luau.Set:
		return renderSet(s, n)
	case *luau.MixedTable:
		return renderMixedTable(s, n)
	// statements
	case *luau.Assignment:
		return renderAssignment(s, n)
	case *luau.BreakStatement:
		return s.Line("break")
	case *luau.CallStatement:
		return renderCallStatement(s, n)
	case *luau.ContinueStatement:
		return s.Line("continue")
	case *luau.DoStatement:
		return renderDoStatement(s, n)
	case *luau.WhileStatement:
		return renderWhileStatement(s, n)
	case *luau.RepeatStatement:
		return renderRepeatStatement(s, n)
	case *luau.IfStatement:
		return renderIfStatement(s, n)
	case *luau.NumericForStatement:
		return renderNumericForStatement(s, n)
	case *luau.ForStatement:
		return renderForStatement(s, n)
	case *luau.FunctionDeclaration:
		return renderFunctionDeclaration(s, n)
	case *luau.MethodDeclaration:
		return renderMethodDeclaration(s, n)
	case *luau.VariableDeclaration:
		return renderVariableDeclaration(s, n)
	case *luau.ReturnStatement:
		return renderReturnStatement(s, n)
	case *luau.Comment:
		return renderComment(s, n)
	// fields
	case *luau.MapField:
		return renderMapField(s, n)
	case *luau.InterpolatedStringPart:
		return renderInterpolatedStringPart(s, n)
	}
	panic("render: no renderer for " + node.Kind().String())
}
