package render

import (
	"strings"

	"rotor/internal/luau"
)

func renderExprOrList(s *RenderState, v luau.NodeOrList) string {
	switch x := v.(type) {
	case *luau.List[luau.Expression]:
		if x.IsEmpty() {
			panic("empty expression list")
		}
		parts := []string{}
		x.ForEach(func(e luau.Expression) { parts = append(parts, Render(s, e)) })
		return strings.Join(parts, ", ")
	case *luau.List[luau.WritableExpression]:
		if x.IsEmpty() {
			panic("empty writable expression list")
		}
		parts := []string{}
		x.ForEach(func(e luau.WritableExpression) { parts = append(parts, Render(s, e)) })
		return strings.Join(parts, ", ")
	case *luau.List[luau.AnyIdentifier]:
		if x.IsEmpty() {
			panic("empty identifier list")
		}
		parts := []string{}
		x.ForEach(func(e luau.AnyIdentifier) { parts = append(parts, Render(s, e)) })
		return strings.Join(parts, ", ")
	case luau.Node:
		return Render(s, x)
	}
	panic("renderExprOrList: unsupported value")
}

// renderReturnStatement.ts
func renderReturnStatement(s *RenderState, node *luau.ReturnStatement) string {
	return s.Line("return " + renderExprOrList(s, node.Expression))
}

// The renderers below are implemented in Task 14.

func renderAssignment(s *RenderState, n *luau.Assignment) string { panic("TODO Task 14") }

func renderCallStatement(s *RenderState, n *luau.CallStatement) string { panic("TODO Task 14") }

func renderComment(s *RenderState, n *luau.Comment) string { panic("TODO Task 14") }

func renderDoStatement(s *RenderState, n *luau.DoStatement) string { panic("TODO Task 14") }

func renderWhileStatement(s *RenderState, n *luau.WhileStatement) string { panic("TODO Task 14") }

func renderRepeatStatement(s *RenderState, n *luau.RepeatStatement) string { panic("TODO Task 14") }

func renderIfStatement(s *RenderState, n *luau.IfStatement) string { panic("TODO Task 14") }

func renderNumericForStatement(s *RenderState, n *luau.NumericForStatement) string {
	panic("TODO Task 14")
}

func renderForStatement(s *RenderState, n *luau.ForStatement) string { panic("TODO Task 14") }

func renderFunctionDeclaration(s *RenderState, n *luau.FunctionDeclaration) string {
	panic("TODO Task 14")
}

func renderMethodDeclaration(s *RenderState, n *luau.MethodDeclaration) string {
	panic("TODO Task 14")
}

func renderVariableDeclaration(s *RenderState, n *luau.VariableDeclaration) string {
	panic("TODO Task 14")
}
