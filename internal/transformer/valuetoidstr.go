package transformer

import (
	"strings"

	"rotor/internal/luau"
)

// expressionToStr ports util/valueToIdStr.ts expressionToStr. The second
// return mirrors upstream's `string | undefined`.
func expressionToStr(expression luau.Expression) (string, bool) {
	switch e := expression.(type) {
	case *luau.Identifier:
		// X -> "X" (note: TemporaryIdentifier is NOT luau.isIdentifier upstream)
		return e.Name, true
	case *luau.PropertyAccessExpression:
		// A.B -> "B"
		return e.Name, true
	case *luau.CallExpression:
		// X.new() -> "X"
		if prop, ok := e.Expression.(*luau.PropertyAccessExpression); ok && prop.Name == "new" {
			return expressionToStr(prop.Expression)
		}
	}
	return "", false
}

// uncapitalizeFirstLetter ports upstream: charAt(0).toLowerCase() + slice(1).
// The input has already passed luau.IsValidIdentifier, so the first character
// is ASCII [A-Za-z_] and byte-wise lowering matches JS exactly.
func uncapitalizeFirstLetter(str string) string {
	if str == "" {
		return str
	}
	return strings.ToLower(str[:1]) + str[1:]
}

// ValueToIdStr ports util/valueToIdStr.ts valueToIdStr: derives a temp-id
// name hint from an expression, or "" for an anonymous temp.
func ValueToIdStr(value luau.Expression) string {
	if valueStr, ok := expressionToStr(value); ok && luau.IsValidIdentifier(valueStr) {
		return uncapitalizeFirstLetter(valueStr)
	}
	return ""
}
