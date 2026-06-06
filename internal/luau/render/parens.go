package render

import "rotor/internal/luau"

// Luau operator precedence — https://www.lua.org/manual/5.1/manual.html#2.5.6
const ifExpressionPrecedence = 1

var unaryOperatorPrecedence = map[luau.UnaryOperator]int{
	"not": 7, "#": 7, "-": 7,
}

var binaryOperatorPrecedence = map[luau.BinaryOperator]int{
	"or": 1, "and": 2,
	"<": 3, ">": 3, "<=": 3, ">=": 3, "~=": 3, "==": 3,
	"..": 4, "+": 5, "-": 5,
	"*": 6, "/": 6, "//": 6, "%": 6,
	"^": 8,
}

func getPrecedence(node luau.Node) int {
	switch n := node.(type) {
	case *luau.IfExpression:
		return ifExpressionPrecedence
	case *luau.BinaryExpression:
		return binaryOperatorPrecedence[n.Operator]
	case *luau.UnaryExpression:
		return unaryOperatorPrecedence[n.Operator]
	}
	panic("getPrecedence: not an expression with precedence")
}

func needsParentheses(node luau.Node) bool {
	parent := node.Parent()
	if parent != nil && luau.IsExpressionWithPrecedence(parent) {
		nodePrec, parentPrec := getPrecedence(node), getPrecedence(parent)
		if nodePrec < parentPrec {
			return true
		}
		if nodePrec == parentPrec {
			if bin, ok := parent.(*luau.BinaryExpression); ok {
				return node == luau.Node(bin.Right)
			}
		}
	}
	return false
}
