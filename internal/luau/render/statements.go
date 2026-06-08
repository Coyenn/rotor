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

// renderAssignment.ts
func renderAssignment(s *RenderState, node *luau.Assignment) string {
	leftStr := renderExprOrList(s, node.Left)
	rightStr := renderExprOrList(s, node.Right)
	return s.LineWithEnd(leftStr+" "+string(node.Operator)+" "+rightStr, node)
}

// renderCallStatement.ts
func renderCallStatement(s *RenderState, node *luau.CallStatement) string {
	return s.LineWithEnd(Render(s, node.Expression), node)
}

// renderComment.ts
func renderComment(s *RenderState, node *luau.Comment) string {
	text := strings.ReplaceAll(node.Text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")
	if len(lines) > 1 {
		eqStr := getSafeBracketEquals(text)
		result := s.Line("--[" + eqStr + "[")
		result += s.Block(func() string {
			var b strings.Builder
			for _, line := range lines {
				b.WriteString(s.Line(line))
			}
			return b.String()
		})
		result += s.Line("]" + eqStr + "]")
		return result
	}
	return s.Line("--" + text)
}

// renderDoStatement.ts
func renderDoStatement(s *RenderState, node *luau.DoStatement) string {
	result := s.Line("do")
	result += s.Block(func() string { return renderStatements(s, node.Statements) })
	result += s.Line("end")
	return result
}

// renderWhileStatement.ts
func renderWhileStatement(s *RenderState, node *luau.WhileStatement) string {
	result := s.Line("while " + Render(s, node.Condition) + " do")
	result += s.Block(func() string { return renderStatements(s, node.Statements) })
	result += s.Line("end")
	return result
}

// renderRepeatStatement.ts
func renderRepeatStatement(s *RenderState, node *luau.RepeatStatement) string {
	result := s.Line("repeat")
	result += s.Block(func() string { return renderStatements(s, node.Statements) })
	result += s.Line("until " + Render(s, node.Condition))
	return result
}

// renderIfStatement.ts
func renderIfStatement(s *RenderState, node *luau.IfStatement) string {
	result := s.Line("if " + Render(s, node.Condition) + " then")
	result += s.Block(func() string { return renderStatements(s, node.Statements) })

	currentElseBody := node.ElseBody
	for {
		ifStmt, ok := currentElseBody.(*luau.IfStatement)
		if !ok {
			break
		}
		statements := ifStmt.Statements
		result += s.Line("elseif " + Render(s, ifStmt.Condition) + " then")
		result += s.Block(func() string { return renderStatements(s, statements) })
		currentElseBody = ifStmt.ElseBody
	}

	if elseList, ok := currentElseBody.(*luau.List[luau.Statement]); ok && elseList.IsNonEmpty() {
		result += s.Line("else")
		result += s.Block(func() string { return renderStatements(s, elseList) })
	}

	result += s.Line("end")
	return result
}

// renderNumericForStatement.ts
func renderNumericForStatement(s *RenderState, node *luau.NumericForStatement) string {
	predicateStr := Render(s, node.ID) + " = " + Render(s, node.Start) + ", " + Render(s, node.End)
	if node.Step != nil {
		isOne := false
		if lit, ok := node.Step.(*luau.NumberLiteral); ok {
			if f, err := luau.JSNumberParse(lit.Value); err == nil && f == 1 {
				isOne = true
			}
		}
		if !isOne {
			predicateStr += ", " + Render(s, node.Step)
		}
	}
	result := s.Line("for " + predicateStr + " do")
	result += s.Block(func() string { return renderStatements(s, node.Statements) })
	result += s.Line("end")
	return result
}

// renderForStatement.ts
func renderForStatement(s *RenderState, node *luau.ForStatement) string {
	parts := []string{}
	node.IDs.ForEach(func(id luau.AnyIdentifier) { parts = append(parts, Render(s, id)) })
	idsStr := strings.Join(parts, ", ")
	if idsStr == "" {
		idsStr = "_"
	}
	result := s.Line("for " + idsStr + " in " + Render(s, node.Expression) + " do")
	result += s.Block(func() string { return renderStatements(s, node.Statements) })
	result += s.Line("end")
	return result
}

// renderFunctionDeclaration.ts
func renderFunctionDeclaration(s *RenderState, node *luau.FunctionDeclaration) string {
	if node.Localize {
		if _, ok := node.Name.(luau.AnyIdentifier); !ok {
			panic("local function cannot be a property")
		}
	}
	nameStr := Render(s, node.Name)
	localStr := ""
	if node.Localize {
		localStr = "local "
	}
	result := s.Line(localStr + "function " + nameStr + "(" + renderParameters(s, node) + ")")
	result += s.Block(func() string { return renderStatements(s, node.Statements) })
	result += s.Line("end")
	return result
}

// renderMethodDeclaration.ts
func renderMethodDeclaration(s *RenderState, node *luau.MethodDeclaration) string {
	result := s.Line("function " + Render(s, node.Expression) + ":" + node.Name + "(" + renderParameters(s, node) + ")")
	result += s.Block(func() string { return renderStatements(s, node.Statements) })
	result += s.Line("end")
	return result
}

// renderVariableDeclaration.ts
func renderVariableDeclaration(s *RenderState, node *luau.VariableDeclaration) string {
	leftStr := renderExprOrList(s, node.Left)
	if node.Right != nil {
		rightStr := renderExprOrList(s, node.Right)
		return s.LineWithEnd("local "+leftStr+" = "+rightStr, node)
	}
	return s.LineWithEnd("local "+leftStr, node)
}
