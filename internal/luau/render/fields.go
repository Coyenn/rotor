package render

import (
	"regexp"

	"rotor/internal/luau"
)

// renderMapField.ts
func renderMapField(s *RenderState, node *luau.MapField) string {
	valueStr := Render(s, node.Value)
	if str, ok := node.Index.(*luau.StringLiteral); ok && luau.IsValidIdentifier(str.Value) {
		return str.Value + " = " + valueStr
	}
	return "[" + Render(s, node.Index) + "] = " + valueStr
}

// renderInterpolatedStringPart.ts
var (
	bracePartRegex   = regexp.MustCompile(`(\\u\{[a-fA-F0-9]+\})|([{}])`)
	newlinePartRegex = regexp.MustCompile("(\r\n?|\n)")
)

func renderInterpolatedStringPart(s *RenderState, node *luau.InterpolatedStringPart) string {
	// escape braces, but do not touch braces within unicode escape codes
	text := bracePartRegex.ReplaceAllStringFunc(node.Text, func(m string) string {
		if m == "{" || m == "}" {
			return "\\" + m
		}
		return m // unicode escape untouched
	})
	// escape newlines, captures a CR with optionally an LF after it or just an LF on its own
	return newlinePartRegex.ReplaceAllString(text, "\\$1")
}
