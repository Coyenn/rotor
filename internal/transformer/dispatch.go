package transformer

import (
	"rotor/internal/luau"
	"rotor/tsgo/ast"
	"rotor/tsgo/checker"
)

func init() {
	// Wire the statement dispatcher into the func var TransformStatementList
	// consults (declared in statementlist.go to break the cycle).
	TransformStatement = transformStatementDispatch
}

// TransformExpression ports nodes/expressions/transformExpression.ts: table
// dispatch on node.kind. Banned kinds add their upstream diagnostic and
// return luau.NewNone() (NO_EMIT) — diagnostics never abort the walk.
// Expression kinds upstream supports but rotor has not ported yet raise
// DiagRotorNotYetSupported instead of upstream's `assert(false)`.
func TransformExpression(s *State, node *ast.Node) luau.Expression {
	switch node.Kind {
	// banned expressions
	case ast.KindBigIntLiteral:
		s.Diags.Add(DiagNoBigInt(node))
		return luau.NewNone()
	case ast.KindNullKeyword:
		s.Diags.Add(DiagNoNullLiteral(node))
		return luau.NewNone()
	case ast.KindPrivateIdentifier:
		s.Diags.Add(DiagNoPrivateIdentifier(node))
		return luau.NewNone()
	case ast.KindRegularExpressionLiteral:
		s.Diags.Add(DiagNoRegex(node))
		return luau.NewNone()
	case ast.KindTypeOfExpression:
		s.Diags.Add(DiagNoTypeOfExpression(node))
		return luau.NewNone()

	// skip transforms
	case ast.KindImportKeyword:
		return luau.NewNone()

	// regular transforms
	case ast.KindArrayLiteralExpression:
		return transformArrayLiteralExpression(s, node)
	case ast.KindBinaryExpression:
		return transformBinaryExpression(s, node)
	case ast.KindCallExpression:
		return transformCallExpression(s, node)
	case ast.KindFalseKeyword:
		return luau.Bool(false)
	case ast.KindTrueKeyword:
		return luau.Bool(true)
	case ast.KindIdentifier:
		return TransformIdentifier(s, node)
	case ast.KindNoSubstitutionTemplateLiteral:
		return transformNoSubstitutionTemplateLiteral(s, node)
	case ast.KindNumericLiteral:
		return transformNumericLiteral(s, node)
	case ast.KindObjectLiteralExpression:
		return transformObjectLiteralExpression(s, node)
	case ast.KindParenthesizedExpression:
		return transformParenthesizedExpression(s, node)
	case ast.KindStringLiteral:
		return transformStringLiteral(s, node)
	case ast.KindTemplateExpression:
		return transformTemplateExpression(s, node)

	// type-only wrappers -> transformTypeExpression (inner expression)
	case ast.KindAsExpression,
		ast.KindExpressionWithTypeArguments,
		ast.KindNonNullExpression,
		ast.KindSatisfiesExpression,
		ast.KindTypeAssertionExpression:
		return TransformExpression(s, node.Expression())
	}

	// Upstream-supported kinds awaiting their port (functions, classes, JSX,
	// unary, access, new, await, conditional, ...) and genuinely unknown
	// kinds both fail loudly.
	s.Diags.Add(DiagRotorNotYetSupported(node, kindName(node.Kind)))
	return luau.NewNone()
}

// transformStatementDispatch ports nodes/statements/transformStatement.ts.
func transformStatementDispatch(s *State, node *ast.Node) *luau.List[luau.Statement] {
	// declare-modifier skip: `declare` means the identifier is defined
	// elsewhere; emit nothing (upstream transformStatement L103-107).
	if ast.CanHaveModifiers(node) {
		if modifiers := node.Modifiers(); modifiers != nil {
			for _, modifier := range modifiers.Nodes {
				if modifier.Kind == ast.KindDeclareKeyword {
					return luau.NewList[luau.Statement]()
				}
			}
		}
	}

	switch node.Kind {
	// no emit
	case ast.KindInterfaceDeclaration, ast.KindTypeAliasDeclaration, ast.KindEmptyStatement:
		return luau.NewList[luau.Statement]()

	// banned statements
	case ast.KindForInStatement:
		s.Diags.Add(DiagNoForInStatement(node))
		return luau.NewList[luau.Statement]()
	case ast.KindLabeledStatement:
		s.Diags.Add(DiagNoLabeledStatement(node))
		return luau.NewList[luau.Statement]()
	case ast.KindDebuggerStatement:
		s.Diags.Add(DiagNoDebuggerStatement(node))
		return luau.NewList[luau.Statement]()

	// regular transforms
	case ast.KindVariableStatement:
		return transformVariableStatement(s, node)
	case ast.KindExpressionStatement:
		return transformExpressionStatement(s, node)
	}

	// Remaining upstream-supported statements (blocks, control flow,
	// functions, classes, imports/exports, ...) land with Tasks 8-13.
	s.Diags.Add(DiagRotorNotYetSupported(node, kindName(node.Kind)))
	return luau.NewList[luau.Statement]()
}

// ---------------------------------------------------------------------------
// Calls (minimal path — full call support: Task 10)
// ---------------------------------------------------------------------------

// transformCallExpression handles bare `f(args)` where the callee is an
// Identifier — enough for `print(...)` in the literal fixtures.
// full call support: Task 10 (optional chains, method calls a.b(...), call
// macros, isMethod, expressionMightMutate callee pinning, LuaTuple wrapping,
// fixVoidArgumentsForRobloxFunctions).
func transformCallExpression(s *State, node *ast.Node) luau.Expression {
	call := node.AsCallExpression()
	callee := SkipDownwards(call.Expression)
	if !ast.IsIdentifier(callee) {
		s.Diags.Add(DiagRotorNotYetSupported(node, kindName(call.Expression.Kind)+" calls"))
		return luau.NewNone()
	}

	expression := TransformExpression(s, callee)
	args := luau.NewList[luau.Expression](transformExpressionsLeftToRight(s, call.Arguments.Nodes)...)
	return luau.NewCall(convertToIndexableExpression(expression), args)
}

// convertToIndexableExpression ports util/convertToIndexableExpression.ts:
// wrap non-indexable expressions in parentheses so they can be indexed or
// called.
func convertToIndexableExpression(expression luau.Expression) luau.IndexableExpression {
	if luau.IsIndexableExpression(expression) {
		return expression.(luau.IndexableExpression)
	}
	return luau.NewParenthesized(expression)
}

// transformExpressionsLeftToRight transforms sibling expressions (call args,
// array elements, template spans) in source order. Upstream routes these
// through util/ensureTransformOrder.ts, which additionally pins earlier
// expressions into temps when a later sibling produces prerequisite
// statements; no Task 6 transform produces prereqs, so plain source order is
// behavior-identical here. ensureTransformOrder: Task 10.
func transformExpressionsLeftToRight(s *State, nodes []*ast.Node) []luau.Expression {
	result := make([]luau.Expression, len(nodes))
	for i, node := range nodes {
		result[i] = TransformExpression(s, node)
	}
	return result
}

// ---------------------------------------------------------------------------
// Binary (minimal path — full binary/logical/assignment support: Task 9)
// ---------------------------------------------------------------------------

// transformBinaryExpression ports expressions/transformBinaryExpression.ts
// for the banned-operator diagnostics and the `+` operator (the `..` vs `+`
// decision of util/createBinaryFromOperator.ts createBinaryAdd). Everything
// else — logical, assignment, comparison, arithmetic, bitwise: Task 9.
// validateNotAnyType gating: Task 8 (type predicates).
func transformBinaryExpression(s *State, node *ast.Node) luau.Expression {
	expression := node.AsBinaryExpression()
	operatorKind := expression.OperatorToken.Kind

	switch operatorKind {
	case ast.KindEqualsEqualsToken:
		s.Diags.Add(DiagNoEqualsEquals(node))
		return luau.NewNone()
	case ast.KindExclamationEqualsToken:
		s.Diags.Add(DiagNoExclamationEquals(node))
		return luau.NewNone()
	case ast.KindPlusToken:
		ordered := transformExpressionsLeftToRight(s, []*ast.Node{expression.Left, expression.Right})
		return createBinaryAdd(s, expression, ordered[0], ordered[1])
	}

	s.Diags.Add(DiagRotorNotYetSupported(node, "operator `"+kindName(operatorKind)+"`"))
	return luau.NewNone()
}

// createBinaryAdd ports util/createBinaryFromOperator.ts createBinaryAdd: if
// either side is definitely a string, emit `..` concatenation with
// `tostring()` wrapped around any side that is not definitely a string; else
// numeric `+`.
func createBinaryAdd(s *State, expression *ast.BinaryExpression, left, right luau.Expression) luau.Expression {
	leftIsString := isDefinitelyStringType(s.GetType(expression.Left))
	rightIsString := isDefinitelyStringType(s.GetType(expression.Right))
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

// isDefinitelyStringType is the minimal isDefinitelyType(type, isStringType)
// from util/types.ts: union -> every constituent, intersection -> some
// constituent, leaf -> TypeFlags.StringLike. The full combinators (constraint
// lookup, class/interface base-type recursion) land with Task 8.
func isDefinitelyStringType(t *checker.Type) bool {
	if t.IsUnion() {
		for _, constituent := range t.Types() {
			if !isDefinitelyStringType(constituent) {
				return false
			}
		}
		return true
	}
	if t.IsIntersection() {
		for _, constituent := range t.Types() {
			if isDefinitelyStringType(constituent) {
				return true
			}
		}
		return false
	}
	return t.Flags()&checker.TypeFlagsStringLike != 0
}
