package transformer

import (
	"rotor/internal/luau"
	"rotor/tsgo/ast"
)

// This file ports nodes/expressions/transformConditionalExpression.ts and its
// helper util/isUsedAsStatement.ts.

// isUsedAsStatement ports util/isUsedAsStatement.ts: true when the expression
// (looking through skipUpwards wrappers) sits directly in statement position —
// an ExpressionStatement, a for-statement initializer/incrementor (the
// condition still produces a value), or a delete expression that is itself
// used as a statement.
func isUsedAsStatement(expression *ast.Node) bool {
	child := SkipUpwards(expression)
	parent := child.Parent
	if parent == nil {
		return false
	}

	if ast.IsExpressionStatement(parent) {
		return true
	}

	// if part of for statement definition, except if used as the condition
	if ast.IsForStatement(parent) && parent.AsForStatement().Condition != child {
		return true
	}

	if ast.IsDeleteExpression(parent) && isUsedAsStatement(parent) {
		return true
	}

	return false
}

// transformConditionalExpression ports transformConditionalExpression.ts:
// `a ? b : c`. Statement position renders as a plain if-statement; a
// prereq-free value position inlines to a Luau IfExpression
// (`if a then b else c`); otherwise an undeclared `_result` temp is assigned
// in both branches of an if-statement.
func transformConditionalExpression(s *State, node *ast.Node) luau.Expression {
	conditional := node.AsConditionalExpression()
	condition := TransformExpression(s, conditional.Condition)
	whenTrue, whenTruePrereqs := s.Capture(func() luau.Expression { return TransformExpression(s, conditional.WhenTrue) })
	whenFalse, whenFalsePrereqs := s.Capture(func() luau.Expression { return TransformExpression(s, conditional.WhenFalse) })

	if isUsedAsStatement(node) {
		whenTruePrereqs.PushList(wrapExpressionStatement(whenTrue))
		whenFalsePrereqs.PushList(wrapExpressionStatement(whenFalse))
		s.Prereq(luau.NewIf(
			CreateTruthinessChecks(s, condition, conditional.Condition, s.GetType(conditional.Condition)),
			whenTruePrereqs,
			whenFalsePrereqs,
		))
		return luau.NewNone()
	}

	if whenTruePrereqs.IsEmpty() && whenFalsePrereqs.IsEmpty() {
		return luau.NewIfExpression(
			CreateTruthinessChecks(s, condition, conditional.Condition, s.GetType(conditional.Condition)),
			whenTrue,
			whenFalse,
		)
	}

	tempID := luau.TempID("result")
	s.Prereq(luau.NewVariableDeclaration(tempID, nil))

	whenTruePrereqs.Push(luau.NewAssignment(tempID, "=", whenTrue))
	whenFalsePrereqs.Push(luau.NewAssignment(tempID, "=", whenFalse))

	s.Prereq(luau.NewIf(
		CreateTruthinessChecks(s, condition, conditional.Condition, s.GetType(conditional.Condition)),
		whenTruePrereqs,
		whenFalsePrereqs,
	))

	return tempID
}
