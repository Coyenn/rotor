package transformer

import (
	"rotor/internal/luau"
	"rotor/tsgo/ast"
	"rotor/tsgo/checker"
)

// This file ports util/assignment.ts, util/getAssignableValue.ts, and
// nodes/transformWritable.ts.

// compoundOperatorMap ports assignment.ts COMPOUND_OPERATOR_MAP (L7-21).
// Operators absent here (`&=`, `|=`, `^=`, `<<=`, `>>=`, `>>>=`; the
// logical-assignment ops are routed earlier) have no simple Luau form and
// fall back to read-modify-write via createCompoundAssignment*.
var compoundOperatorMap = map[ast.Kind]luau.AssignmentOperator{
	// compound assignment
	ast.KindMinusEqualsToken:            "-=",
	ast.KindAsteriskEqualsToken:         "*=",
	ast.KindSlashEqualsToken:            "/=",
	ast.KindAsteriskAsteriskEqualsToken: "^=",
	ast.KindPercentEqualsToken:          "%=",

	// unary
	ast.KindPlusPlusToken:   "+=",
	ast.KindMinusMinusToken: "-=",

	// normal assignment
	ast.KindEqualsToken: "=",
}

// getSimpleAssignmentOperator ports assignment.ts getSimpleAssignmentOperator
// (L23-34). `+=` decides `..=` vs `+=` with the same definitely-string test
// as createBinaryAdd. ok=false means no simple operator exists (upstream
// `undefined`) and the caller must use the read-modify-write fallback.
func getSimpleAssignmentOperator(s *State, leftType *checker.Type, operatorKind ast.Kind, rightType *checker.Type) (operator luau.AssignmentOperator, ok bool) {
	// plus
	if operatorKind == ast.KindPlusEqualsToken {
		if IsDefinitelyType(s, leftType, IsStringType) || IsDefinitelyType(s, rightType, IsStringType) {
			return "..=", true
		}
		return "+=", true
	}

	operator, ok = compoundOperatorMap[operatorKind]
	return operator, ok
}

// getAssignableValue ports util/getAssignableValue.ts (L5-10): a `..=` value
// that is not definitely a string gets wrapped in tostring() (mirrors
// createBinaryAdd's coercion for the `+=` case).
func getAssignableValue(s *State, operator luau.AssignmentOperator, value luau.Expression, valueType *checker.Type) luau.Expression {
	if operator == "..=" && !IsDefinitelyType(s, valueType, IsStringType) {
		return luau.NewCall(luau.GlobalID("tostring"), luau.NewList[luau.Expression](value))
	}
	return value
}

// createAssignmentExpression ports assignment.ts createAssignmentExpression
// (L36-50): prereq the assignment, return the writable as the expression's
// value.
func createAssignmentExpression(s *State, readable luau.WritableExpression, operator luau.AssignmentOperator, value luau.Expression) luau.Expression {
	s.Prereq(luau.NewAssignment(readable, operator, value))
	return readable
}

// createCompoundAssignmentStatement ports assignment.ts
// createCompoundAssignmentStatement (L52-67): read-modify-write
// `writable = <binary(readable, op, value)>`. (Upstream also threads the TS
// node through to createBinaryFromOperator, which never reads it.)
func createCompoundAssignmentStatement(s *State, writable luau.WritableExpression, writableType *checker.Type, readable luau.WritableExpression, operatorKind ast.Kind, value luau.Expression, valueType *checker.Type) luau.Statement {
	return luau.NewAssignment(
		writable,
		"=",
		createBinaryFromOperator(s, readable, writableType, operatorKind, value, valueType),
	)
}

// createCompoundAssignmentExpression ports assignment.ts
// createCompoundAssignmentExpression (L69-85).
func createCompoundAssignmentExpression(s *State, writable luau.WritableExpression, writableType *checker.Type, readable luau.WritableExpression, operatorKind ast.Kind, value luau.Expression, valueType *checker.Type) luau.Expression {
	return createAssignmentExpression(
		s,
		writable,
		"=",
		createBinaryFromOperator(s, readable, writableType, operatorKind, value, valueType),
	)
}

// transformWritableExpression ports transformWritable.ts
// transformWritableExpression (L13-41) for identifier lvalues. The
// PropertyAccessExpression branch (pushToVarIfNonId base pinning) and
// ElementAccessExpression branch (ensureTransformOrder + addOneIfArrayType
// index offsetting + index pinning) need the access transforms: Task 10 —
// until then property/element lvalues raise rotorNotYetSupported and a
// discardable temp stands in for the writable.
func transformWritableExpression(s *State, node *ast.Node, readAfterWrite bool) luau.WritableExpression {
	if ast.IsPrototypeAccess(node) {
		s.Diags.Add(DiagNoPrototype(node))
	}
	if ast.IsPropertyAccessExpression(node) || ast.IsElementAccessExpression(node) {
		// property/element access lvalues: Task 10.
		s.Diags.Add(DiagRotorNotYetSupported(node, "assignment to "+kindName(node.Kind)))
		return luau.TempID("")
	}
	transformed := TransformExpression(s, SkipDownwards(node))
	writable, ok := transformed.(luau.WritableExpression)
	if !ok {
		panic("transformer: transformWritableExpression: lvalue is not writable") // upstream assert
	}
	return writable
}

// writableAssignment carries transformWritableAssignment's result triple.
// readable is a pre-RHS snapshot of the target for the compound fallback;
// writable is the assignment target; value is the transformed RHS.
type writableAssignment struct {
	writable luau.WritableExpression
	readable luau.WritableExpression
	value    luau.Expression
}

// transformWritableAssignment ports transformWritable.ts
// transformWritableAssignment (L43-58). readable only materializes into a
// `readable` temp when the RHS had prereqs that could mutate the target (and
// the caller actually reads before writing).
func transformWritableAssignment(s *State, writeNode, valueNode *ast.Node, readAfterWrite, readBeforeWrite bool) writableAssignment {
	writable := transformWritableExpression(s, writeNode, readAfterWrite)
	value, prereqs := s.Capture(func() luau.Expression { return TransformExpression(s, valueNode) })

	// if !readBeforeWrite, readable won't be used anyways
	readable := writable
	if readBeforeWrite && prereqs.IsNonEmpty() {
		readable = s.PushToVar(writable, "readable")
	}
	s.PrereqList(prereqs)

	return writableAssignment{writable: writable, readable: readable, value: value}
}
