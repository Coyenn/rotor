package transformer

import (
	"rotor/internal/luau"
	"rotor/tsgo/ast"
)

// ---------------------------------------------------------------------------
// for-of — statements/transformForOfStatement.ts
// ---------------------------------------------------------------------------
//
// Pulled forward from the for-of task (Task 5) because the functions fixture
// iterates a rest-parameter array (`for (const n of nums)`). Only the ARRAY
// loop builder is live; the remaining builders (Set/Map/String/
// IterableFunction/Generator) and the `$range` numeric-for macro land with
// Task 5's full dispatch table and raise rotorNotYetSupported until then.
// (A `$range(...)` expression resolves to a compiler-types symbol, so the
// Phase 2 macro stand-in already diagnoses it inside TransformExpression
// before the builder dispatch is reached.)

// transformForInitializer ports transformForOfStatement.ts
// transformForInitializer (L94-128): produce the single identifier bound by
// the loop, prepending any binding statements to the body via initializers.
func transformForInitializer(s *State, initializer *ast.Node, initializers *luau.List[luau.Statement]) luau.AnyIdentifier {
	if ast.IsVariableDeclarationList(initializer) {
		return transformBindingName(s, initializer.AsVariableDeclarationList().Declarations.Nodes[0].Name(), initializers)
	} else if ast.IsArrayLiteralExpression(initializer) || ast.IsObjectLiteralExpression(initializer) {
		// transformArray/ObjectAssignmentPattern over a `_binding` temp:
		// destructuring task.
		s.Diags.Add(DiagRotorNotYetSupported(initializer, kindName(initializer.Kind)))
		return luau.TempID("binding")
	}
	// `for (x of ...)`, `for (a.b of ...)` — a `_v` temp is the loop binding,
	// assigned through the writable target at the top of the body.
	valueID := luau.TempID("v")
	expression := transformWritableExpression(s, initializer, false)
	initializers.Push(luau.NewAssignment(expression, "=", valueID))
	return valueID
}

// transformBindingName ports nodes/binding/transformBindingName.ts (L8-30).
// Binding patterns (`for (const [a, b] of ...)`) destructure a `_binding`
// temp inside the body: destructuring task.
func transformBindingName(s *State, name *ast.Node, initializers *luau.List[luau.Statement]) luau.AnyIdentifier {
	if ast.IsIdentifier(name) {
		return TransformIdentifierDefined(s, name)
	}
	s.Diags.Add(DiagRotorNotYetSupported(name, kindName(name.Kind)))
	return luau.TempID("binding")
}

// buildArrayLoop ports transformForOfStatement.ts buildArrayLoop (L130-134)
// through makeForLoopBuilder (L41-57): `for _, x in exp do ... end` — the
// first generic-for binding is an unnamed temp (renders `_` when
// unreferenced); there is no ipairs in 3.0, generalized iteration directly
// over the array expression.
func buildArrayLoop(s *State, statements *luau.List[luau.Statement], initializer *ast.Node, exp luau.Expression) *luau.List[luau.Statement] {
	ids := luau.NewList[luau.AnyIdentifier]()
	initializers := luau.NewList[luau.Statement]()

	ids.Push(luau.TempID(""))
	ids.Push(transformForInitializer(s, initializer, initializers))

	statements.UnshiftList(initializers)
	return luau.NewList[luau.Statement](luau.NewFor(ids, exp, statements))
}

// transformForOfStatement ports transformForOfStatement.ts (L479-508). The
// body is transformed BEFORE the loop shape is built; builders prepend
// initializer statements.
func transformForOfStatement(s *State, node *ast.Node) *luau.List[luau.Statement] {
	forOf := node.AsForInOrOfStatement()

	if forOf.AwaitModifier != nil {
		s.Diags.Add(DiagNoAwaitForOf(node))
	}

	if ast.IsVariableDeclarationList(forOf.Initializer) {
		name := forOf.Initializer.AsVariableDeclarationList().Declarations.Nodes[0].Name()
		if ast.IsIdentifier(name) {
			ValidateIdentifier(s, name)
		}
	}

	result := luau.NewList[luau.Statement]()

	var exp luau.Expression
	expPrereqs := s.CaptureStatements(func() {
		exp = TransformExpression(s, forOf.Expression)
	})
	result.PushList(expPrereqs)

	expType := s.GetType(forOf.Expression)
	statements := TransformStatementList(s, forOf.Statement, getStatements(forOf.Statement), nil)

	// getLoopBuilder (L414-438): array only — the remaining entries land with
	// the for-of task.
	if IsDefinitelyType(s, expType, IsArrayType(s)) {
		result.PushList(buildArrayLoop(s, statements, forOf.Initializer, exp))
	} else {
		s.Diags.Add(DiagRotorNotYetSupported(forOf.Expression, "for-of over non-array types"))
	}

	return result
}
