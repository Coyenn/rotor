package transformer

import (
	"rotor/internal/luau"
	"rotor/tsgo/ast"
	"rotor/tsgo/checker"
)

// This file ports expressions/transformBinaryExpression.ts and
// util/createBinaryFromOperator.ts.

// operatorMap ports createBinaryFromOperator.ts OPERATOR_MAP (L9-24): TS
// binary operator tokens with a 1:1 Luau binary operator.
var operatorMap = map[ast.Kind]luau.BinaryOperator{
	// comparison
	ast.KindLessThanToken:                "<",
	ast.KindGreaterThanToken:             ">",
	ast.KindLessThanEqualsToken:          "<=",
	ast.KindGreaterThanEqualsToken:       ">=",
	ast.KindEqualsEqualsEqualsToken:      "==",
	ast.KindExclamationEqualsEqualsToken: "~=",

	// math
	ast.KindMinusToken:            "-",
	ast.KindAsteriskToken:         "*",
	ast.KindSlashToken:            "/",
	ast.KindAsteriskAsteriskToken: "^",
	ast.KindPercentToken:          "%",
}

// bitwiseOperatorMap ports createBinaryFromOperator.ts BITWISE_OPERATOR_MAP
// (L26-42): TS bitwise tokens to bit32 function names. The `=`-suffixed
// compound forms reach this map through the read-modify-write fallback
// (createCompoundAssignment* passes the compound token straight through).
var bitwiseOperatorMap = map[ast.Kind]string{
	// bitwise
	ast.KindAmpersandToken:                         "band",
	ast.KindBarToken:                               "bor",
	ast.KindCaretToken:                             "bxor",
	ast.KindLessThanLessThanToken:                  "lshift",
	ast.KindGreaterThanGreaterThanGreaterThanToken: "rshift",
	ast.KindGreaterThanGreaterThanToken:            "arshift",

	// bitwise compound assignment
	ast.KindAmpersandEqualsToken:                         "band",
	ast.KindBarEqualsToken:                               "bor",
	ast.KindCaretEqualsToken:                             "bxor",
	ast.KindLessThanLessThanEqualsToken:                  "lshift",
	ast.KindGreaterThanGreaterThanGreaterThanEqualsToken: "rshift",
	ast.KindGreaterThanGreaterThanEqualsToken:            "arshift",
}

// createBinaryAdd ports createBinaryFromOperator.ts createBinaryAdd (L44-56)
// — THE `..` vs `+` decision: if either side is definitely a string, emit
// `..` concatenation with `tostring()` wrapped around any side that is not
// definitely a string; else numeric `+`.
func createBinaryAdd(s *State, left luau.Expression, leftType *checker.Type, right luau.Expression, rightType *checker.Type) luau.Expression {
	leftIsString := IsDefinitelyType(s, leftType, IsStringType)
	rightIsString := IsDefinitelyType(s, rightType, IsStringType)
	if leftIsString || rightIsString {
		if !leftIsString {
			left = luau.NewCall(luau.GlobalID("tostring"), luau.NewList[luau.Expression](left))
		}
		if !rightIsString {
			right = luau.NewCall(luau.GlobalID("tostring"), luau.NewList[luau.Expression](right))
		}
		return luau.NewBinary(left, "..", right)
	}
	return luau.NewBinary(left, "+", right)
}

// createBinaryFromOperator ports createBinaryFromOperator.ts (L58-90).
// Resolution order: simple map -> plus -> bitwise call -> comma ->
// assert-unreachable.
func createBinaryFromOperator(s *State, left luau.Expression, leftType *checker.Type, operatorKind ast.Kind, right luau.Expression, rightType *checker.Type) luau.Expression {
	// simple
	if operator, ok := operatorMap[operatorKind]; ok {
		return luau.NewBinary(left, operator, right)
	}

	// plus
	if operatorKind == ast.KindPlusToken || operatorKind == ast.KindPlusEqualsToken {
		return createBinaryAdd(s, left, leftType, right, rightType)
	}

	// bitwise
	if bit32Name, ok := bitwiseOperatorMap[operatorKind]; ok {
		return luau.NewCall(luau.GlobalProperty("bit32", bit32Name), luau.NewList[luau.Expression](left, right))
	}

	if operatorKind == ast.KindCommaToken {
		s.PrereqList(wrapExpressionStatement(left))
		return right
	}

	panic("transformer: createBinaryFromOperator unknown operator: " + kindName(operatorKind)) // upstream assert(false)
}

// transformBinaryExpression ports transformBinaryExpression.ts (L113-253).
// validateNotAnyType on both operands: needs the isArrayType macro-symbol
// predicate — Task 10.
func transformBinaryExpression(s *State, node *ast.Node) luau.Expression {
	expression := node.AsBinaryExpression()
	operatorKind := expression.OperatorToken.Kind

	// banned
	if operatorKind == ast.KindEqualsEqualsToken {
		s.Diags.Add(DiagNoEqualsEquals(node))
		return luau.NewNone()
	} else if operatorKind == ast.KindExclamationEqualsToken {
		s.Diags.Add(DiagNoExclamationEquals(node))
		return luau.NewNone()
	}

	// logical
	if operatorKind == ast.KindAmpersandAmpersandToken ||
		operatorKind == ast.KindBarBarToken ||
		operatorKind == ast.KindQuestionQuestionToken {
		return transformLogical(s, node)
	}

	// transformLogicalOrCoalescingAssignmentExpression (&&=, ||=, ??=): later
	// task (out of Phase 2 first-wave scope).
	if ast.IsLogicalOrCoalescingAssignmentExpression(node) {
		s.Diags.Add(DiagRotorNotYetSupported(node, "operator `"+kindName(operatorKind)+"`"))
		return luau.NewNone()
	}

	if ast.IsAssignmentOperator(operatorKind) {
		// Destructuring assignment (ArrayLiteral/ObjectLiteral LHS, upstream
		// L143-184 incl. the LuaTuple optimized-unpack paths): destructuring
		// task.
		if ast.IsArrayLiteralExpression(expression.Left) || ast.IsObjectLiteralExpression(expression.Left) {
			s.Diags.Add(DiagRotorNotYetSupported(node, "destructuring assignment"))
			return luau.NewNone()
		}

		writableType := s.GetType(expression.Left)
		valueType := s.GetType(expression.Right)
		operator, isSimple := getSimpleAssignmentOperator(s, writableType, operatorKind, valueType)
		assignment := transformWritableAssignment(s, expression.Left, expression.Right, true, !isSimple)
		if isSimple {
			return createAssignmentExpression(s, assignment.writable, operator, getAssignableValue(s, operator, assignment.value, valueType))
		}
		return createCompoundAssignmentExpression(s, assignment.writable, writableType, assignment.readable, operatorKind, assignment.value, valueType)
	}

	ordered := ensureTransformOrder(s, []*ast.Node{expression.Left, expression.Right})
	left, right := ordered[0], ordered[1]

	if operatorKind == ast.KindInKeyword {
		return luau.NewBinary(
			luau.NewComputedIndex(convertToIndexableExpression(right), left),
			"~=",
			luau.Nil(),
		)
	} else if operatorKind == ast.KindInstanceOfKeyword {
		// Upstream gates a noRobloxSymbolInstanceof diagnostic on
		// isPossiblyType(rightType, isRobloxType(state)); isRobloxType tests
		// whether the symbol is declared under node_modules/@rbxts/types,
		// which needs the project-layer nodeModulesPath — Phase 4 (the
		// runtime-lib call below already aborts compilation until then).
		return luau.NewCall(s.RuntimeLib(node, "instanceof"), luau.NewList[luau.Expression](left, right))
	}

	leftType := s.GetType(expression.Left)
	rightType := s.GetType(expression.Right)

	if operatorKind == ast.KindLessThanToken ||
		operatorKind == ast.KindLessThanEqualsToken ||
		operatorKind == ast.KindGreaterThanToken ||
		operatorKind == ast.KindGreaterThanEqualsToken {
		// NOTE: verbatim upstream quirk (transformBinaryExpression.ts
		// L244-247): the second clause re-tests LEFT type for number where
		// symmetry suggests rightType — ported as-is, byte parity over sanity.
		if (!IsDefinitelyType(s, leftType, IsStringType) && !IsDefinitelyType(s, leftType, IsNumberType)) ||
			(!IsDefinitelyType(s, rightType, IsStringType) && !IsDefinitelyType(s, leftType, IsNumberType)) {
			s.Diags.Add(DiagNoNonNumberStringRelationOperator(node))
		}
	}

	return createBinaryFromOperator(s, left, leftType, operatorKind, right, rightType)
}
