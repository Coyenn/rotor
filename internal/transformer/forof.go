package transformer

import (
	"rotor/internal/luau"
	"rotor/tsgo/ast"
	"rotor/tsgo/checker"
)

// ---------------------------------------------------------------------------
// for-of — statements/transformForOfStatement.ts (COMPLETE dispatch;
// Phase 2b ships the ARRAY builder live)
// ---------------------------------------------------------------------------

// loopBuilder ports `type LoopBuilder` (L34-39): statements is the
// already-transformed body; the builder wraps it in the loop shape.
type loopBuilder func(s *State, statements *luau.List[luau.Statement], initializer *ast.Node, exp luau.Expression) *luau.List[luau.Statement]

// makeForLoopBuilder ports makeForLoopBuilder (L41-57): the callback fills
// `ids` (the generic-for binding list) and `initializers` (statements to
// prepend to the body) and returns the for-expression.
func makeForLoopBuilder(callback func(s *State, initializer *ast.Node, exp luau.Expression, ids *luau.List[luau.AnyIdentifier], initializers *luau.List[luau.Statement]) luau.Expression) loopBuilder {
	return func(s *State, statements *luau.List[luau.Statement], initializer *ast.Node, exp luau.Expression) *luau.List[luau.Statement] {
		ids := luau.NewList[luau.AnyIdentifier]()
		initializers := luau.NewList[luau.Statement]()
		expression := callback(s, initializer, exp, ids, initializers)
		statements.UnshiftList(initializers)
		return luau.NewList[luau.Statement](luau.NewFor(ids, expression, statements))
	}
}

// transformForInitializer ports transformForOfStatement.ts
// transformForInitializer (L94-128): produce the single identifier bound by
// the loop, prepending any binding statements to the body via initializers.
func transformForInitializer(s *State, initializer *ast.Node, initializers *luau.List[luau.Statement]) luau.AnyIdentifier {
	if ast.IsVariableDeclarationList(initializer) {
		return transformBindingName(s, initializer.AsVariableDeclarationList().Declarations.Nodes[0].Name(), initializers)
	} else if ast.IsArrayLiteralExpression(initializer) {
		// `for ([a, b] of ...)` — destructure a `_binding` temp inside the body.
		parentID := luau.TempID("binding")
		initializers.PushList(s.CaptureStatements(func() {
			transformArrayAssignmentPattern(s, initializer, parentID)
		}))
		return parentID
	} else if ast.IsObjectLiteralExpression(initializer) {
		parentID := luau.TempID("binding")
		initializers.PushList(s.CaptureStatements(func() {
			transformObjectAssignmentPattern(s, initializer, parentID)
		}))
		return parentID
	}
	// `for (x of ...)`, `for (a.b of ...)` — a `_v` temp is the loop binding,
	// assigned through the writable target at the top of the body.
	valueID := luau.TempID("v")
	expression := transformWritableExpression(s, initializer, false)
	initializers.Push(luau.NewAssignment(expression, "=", valueID))
	return valueID
}

// buildArrayLoop ports buildArrayLoop (L130-134): `for _, x in exp do` — the
// first generic-for binding is an unnamed temp (renders `_` when
// unreferenced); there is no ipairs in 3.0, generalized iteration directly
// over the array expression. Pattern initializers destructure inside the body
// (`for _, _binding in exp do local a = _binding[1] ... end`).
var buildArrayLoop = makeForLoopBuilder(func(s *State, initializer *ast.Node, exp luau.Expression, ids *luau.List[luau.AnyIdentifier], initializers *luau.List[luau.Statement]) luau.Expression {
	ids.Push(luau.TempID(""))
	ids.Push(transformForInitializer(s, initializer, initializers))
	return exp
})

// emptyLoopBuilder stands in for upstream's `() => luau.list.make()` builder
// returned after a diagnostic was raised at dispatch, and for the
// not-yet-ported builders after their rotorNotYetSupported diagnostic.
func emptyLoopBuilder(s *State, statements *luau.List[luau.Statement], initializer *ast.Node, exp luau.Expression) *luau.List[luau.Statement] {
	return luau.NewList[luau.Statement]()
}

// getLoopBuilder ports getLoopBuilder (L414-438): the isDefinitelyType
// dispatch, in upstream order. Phase 2b implements the array builder; the
// Set/Map/string/IterableFunction/generator builders are pure-Luau emissions
// that land with their Phase 3 types and raise rotorNotYetSupported here
// (from the dispatch, once per loop). Iterable<T> and union keep upstream's
// own noIterableIteration / noMacroUnion errors (both return a builder that
// emits nothing). The fallthrough is upstream's `assert(false, ...)`.
func getLoopBuilder(s *State, node *ast.Node, t *checker.Type) loopBuilder {
	if IsDefinitelyType(s, t, IsArrayType(s)) {
		return buildArrayLoop
	} else if IsDefinitelyType(s, t, IsSetType(s)) {
		// buildSetLoop (L136-139): `for x in exp do`.
		s.Diags.Add(DiagRotorNotYetSupported(node, "for-of over `Set<T>`"))
		return emptyLoopBuilder
	} else if IsDefinitelyType(s, t, IsMapType(s)) {
		// buildMapLoop (L224-257): `for _k, _v in exp do` with the inline
		// `[k, v]`-destructure fast path (transformInLineArrayBindingPattern /
		// transformInLineArrayAssignmentPattern, L141-222).
		s.Diags.Add(DiagRotorNotYetSupported(node, "for-of over `Map<K, V>`"))
		return emptyLoopBuilder
	} else if IsDefinitelyType(s, t, IsStringType) {
		// buildStringLoop (L259-262):
		// `for c in string.gmatch(exp, utf8.charpattern) do`.
		s.Diags.Add(DiagRotorNotYetSupported(node, "for-of over `string`"))
		return emptyLoopBuilder
	} else if IsDefinitelyType(s, t, IsIterableFunctionLuaTupleType(s)) {
		// buildIterableFunctionLuaTupleLoop (L286-382): inline-binding fast
		// path, tuple-arity introspection, or the while-true protocol.
		s.Diags.Add(DiagRotorNotYetSupported(node, "for-of over `IterableFunction<LuaTuple<T>>`"))
		return emptyLoopBuilder
	} else if IsDefinitelyType(s, t, IsIterableFunctionType(s)) {
		// buildIterableFunctionLoop (L264-267): `for x in exp do`.
		s.Diags.Add(DiagRotorNotYetSupported(node, "for-of over `IterableFunction<T>`"))
		return emptyLoopBuilder
	} else if IsDefinitelyType(s, t, IsGeneratorType(s)) {
		// buildGeneratorLoop (L384-412): `for _result in exp.next do` with the
		// done/value protocol.
		s.Diags.Add(DiagRotorNotYetSupported(node, "for-of over generators"))
		return emptyLoopBuilder
	} else if IsDefinitelyType(s, t, IsIterableType(s)) {
		s.Diags.Add(DiagNoIterableIteration(node))
		return emptyLoopBuilder
	} else if t.IsUnion() {
		s.Diags.Add(DiagNoMacroUnion(node))
		return emptyLoopBuilder
	}
	panic("transformer: ForOf iteration type not implemented: " + s.Checker.TypeToString(t)) // upstream assert(false)
}

// findRangeMacro ports findRangeMacro (L440-448): the loop expression
// (skipped downwards) is a call whose callee resolves to the `$range` macro
// symbol. Upstream compares against macroManager.getSymbolOrThrow($range);
// rotor matches the MacroManager call-macro entry by name (same symbol
// identity — the entry was registered from that resolution). Checked
// BEFORE the expression is transformed, matching upstream's position — the
// generic macro sentinel inside TransformExpression is never reached for it.
func findRangeMacro(s *State, node *ast.Node) *ast.Node {
	expression := SkipDownwards(node.AsForInOrOfStatement().Expression)
	if ast.IsCallExpression(expression) {
		symbol := GetFirstDefinedSymbol(s, s.GetType(expression.AsCallExpression().Expression))
		if symbol != nil {
			if macro := s.Macros().GetCallMacro(symbol); macro != nil && macro.Name == "$range" {
				return expression
			}
		}
	}
	return nil
}

// transformForOfStatement ports transformForOfStatement (L479-508). The body
// is transformed BEFORE the loop shape is built; builders prepend initializer
// statements.
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

	if rangeMacroCall := findRangeMacro(s, node); rangeMacroCall != nil {
		// transformForOfRangeMacro (L450-477): the NumericForStatement
		// emission lands with the Phase 3 macro tables.
		s.Diags.Add(DiagRotorNotYetSupported(rangeMacroCall, "macro `$range`"))
		return luau.NewList[luau.Statement]()
	}

	result := luau.NewList[luau.Statement]()

	var exp luau.Expression
	expPrereqs := s.CaptureStatements(func() {
		exp = TransformExpression(s, forOf.Expression)
	})
	result.PushList(expPrereqs)

	expType := s.GetType(forOf.Expression)
	statements := TransformStatementList(s, forOf.Statement, getStatements(forOf.Statement), nil)

	loopBuilder := getLoopBuilder(s, forOf.Expression, expType)
	result.PushList(loopBuilder(s, statements, forOf.Initializer, exp))

	return result
}
