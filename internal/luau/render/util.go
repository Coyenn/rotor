package render

import (
	"strings"

	"rotor/internal/luau"
)

// getSafeBracketEquals returns the `=` padding needed so that `[`+eq+`[` and
// `]`+eq+`]` brackets safely enclose str. Literal port of upstream
// getSafeBracketEquals.ts.
func getSafeBracketEquals(str string) string {
	amtEquals := 0
	for strings.Contains(str, "]"+strings.Repeat("=", amtEquals)+"]") ||
		strings.HasSuffix(str, "]"+strings.Repeat("=", amtEquals)) {
		amtEquals++
	}
	return strings.Repeat("=", amtEquals)
}

func renderArguments(s *RenderState, expressions *luau.List[luau.Expression]) string {
	parts := []string{}
	expressions.ForEach(func(v luau.Expression) { parts = append(parts, Render(s, v)) })
	return strings.Join(parts, ", ")
}

func renderParameters(s *RenderState, node luau.HasParameters) string {
	params, hasDotDotDot := node.ParamData()
	parts := []string{}
	params.ForEach(func(p luau.AnyIdentifier) { parts = append(parts, Render(s, p)) })
	if hasDotDotDot {
		parts = append(parts, "...")
	}
	return strings.Join(parts, ", ")
}

func renderStatements(s *RenderState, statements *luau.List[luau.Statement]) string {
	var b strings.Builder
	hasFinalStatement := false
	for listNode := statements.Head; listNode != nil; listNode = listNode.Next {
		if hasFinalStatement {
			if _, isComment := listNode.Value.(*luau.Comment); !isComment {
				panic("Cannot render statement after break, continue, or return!")
			}
		}
		hasFinalStatement = hasFinalStatement || luau.IsFinalStatement(listNode.Value)
		s.pushListNode(listNode)
		b.WriteString(Render(s, listNode.Value))
		s.popListNode()
	}
	return b.String()
}
