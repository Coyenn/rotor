package render

import (
	"strconv"
	"strings"

	"rotor/internal/luau"
)

// getSafeBracketEquals returns the `=` padding needed so that `[`+eq+`[` and
// `]`+eq+`]` brackets safely enclose str. For every `]` in str followed by a
// run of k `=`s that is then closed by another `]` (or ends the string), a
// bracket level of at least k+1 is required.
func getSafeBracketEquals(str string) string {
	amtEquals := 0
	for i := 0; i < len(str); i++ {
		if str[i] != ']' {
			continue
		}
		k := 0
		for i+1+k < len(str) && str[i+1+k] == '=' {
			k++
		}
		if i+1+k >= len(str) || str[i+1+k] == ']' {
			if k+1 > amtEquals {
				amtEquals = k + 1
			}
		}
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

// parseNumberValue mirrors JS Number(value) for literal forms the compiler emits.
func parseNumberValue(text string) (float64, error) {
	cleaned := strings.ReplaceAll(text, "_", "")
	return strconv.ParseFloat(cleaned, 64)
}
