package transformer

import (
	"rotor/internal/luau"
	"rotor/tsgo/ast"
	"rotor/tsgo/checker"
)

// This file ports the shared destructuring plumbing:
// nodes/binding/transformBindingName.ts, util/binding/
// getAccessorForBindingType.ts, and util/binding/objectAccessor.ts.
// The pattern transforms themselves live in bindingarray.go /
// bindingobject.go.

// ---------------------------------------------------------------------------
// transformBindingName — nodes/binding/transformBindingName.ts (L8-30)
// ---------------------------------------------------------------------------

// transformBindingName produces the single identifier bound by a BindingName:
// identifiers directly, patterns through a `_binding` temp whose destructure
// statements are appended to initializers (used by for-of loop headers;
// variable declarations go through transformVariableDeclaration instead).
func transformBindingName(s *State, name *ast.Node, initializers *luau.List[luau.Statement]) luau.AnyIdentifier {
	if ast.IsIdentifier(name) {
		return TransformIdentifierDefined(s, name)
	}
	id := luau.TempID("binding")
	initializers.PushList(s.CaptureStatements(func() {
		if ast.IsArrayBindingPattern(name) {
			transformArrayBindingPattern(s, name, id)
		} else {
			transformObjectBindingPattern(s, name, id)
		}
	}))
	return id
}

// isOmittedBindingElement is the rotor equivalent of upstream
// `ts.isOmittedExpression(element)` over ArrayBindingPattern elements: the
// tsgo parser represents an array-binding hole (`const [, x] = ...`) as a
// BindingElement with a nil name, not as strada's OmittedExpression node
// (parser.go parseArrayBindingElement: "These are all nil for a missing
// element").
func isOmittedBindingElement(element *ast.Node) bool {
	return ast.IsOmittedExpression(element) ||
		(ast.IsBindingElement(element) && element.AsBindingElement().Name() == nil)
}

// ---------------------------------------------------------------------------
// Accessor table — util/binding/getAccessorForBindingType.ts (COMPLETE table;
// Phase 2b ships the ARRAY accessor live)
// ---------------------------------------------------------------------------

// bindingAccessor ports the BindingAccessor signature: produce the expression
// for one array-position element. idStack carries iteration state BETWEEN
// elements of one pattern (the gmatch matcher, the last `next` key);
// isOmitted=true means the element is hole-only — stateful accessors still
// advance (side-effect prereqs), nothing is bound.
type bindingAccessor func(s *State, parentID luau.AnyIdentifier, index int, idStack *[]luau.AnyIdentifier, isOmitted bool) luau.Expression

// arrayAccessor ports the array entry (L32-37): `parentId[index + 1]` with
// the +1 folded into the literal; an omitted element emits nothing.
func arrayAccessor(s *State, parentID luau.AnyIdentifier, index int, idStack *[]luau.AnyIdentifier, isOmitted bool) luau.Expression {
	return luau.NewComputedIndex(parentID, luau.Num(float64(index+1)))
}

// noneAccessor stands in for the not-yet-ported accessors after their
// diagnostic was raised at dispatch (mirrors upstream's `() => luau.none()`
// for the noIterableIteration entry).
func noneAccessor(s *State, parentID luau.AnyIdentifier, index int, idStack *[]luau.AnyIdentifier, isOmitted bool) luau.Expression {
	return luau.NewNone()
}

// getAccessorForBindingType ports getAccessorForBindingType (L143-167): the
// 8-entry isDefinitelyType dispatch, in upstream order. Phase 2b implements
// the array accessor; the string/Set/Map/IterableFunction/generator-object
// accessors are pure-Luau emissions that land with their Phase 3 types and
// raise rotorNotYetSupported here (from the dispatch, once per pattern — not
// per element). Iterable<T> keeps upstream's own noIterableIteration error.
// The fallthrough is upstream's `assert(false, ...)`.
func getAccessorForBindingType(s *State, node *ast.Node, t *checker.Type) bindingAccessor {
	if IsDefinitelyType(s, t, IsArrayType(s)) {
		return arrayAccessor
	} else if IsDefinitelyType(s, t, IsStringType) {
		// stringAccessor (L39-63): string.gmatch(parentId, utf8.charpattern)
		// matcher pushed to idStack, value = `_matcher()`.
		s.Diags.Add(DiagRotorNotYetSupported(node, "destructuring `string`"))
		return noneAccessor
	} else if IsDefinitelyType(s, t, IsSetType(s)) {
		// setAccessor (L65-84): `next(parentId[, lastValue])` continuation.
		s.Diags.Add(DiagRotorNotYetSupported(node, "destructuring `Set<T>`"))
		return noneAccessor
	} else if IsDefinitelyType(s, t, IsMapType(s)) {
		// mapAccessor (L86-103): `local _k, _v = next(parentId[, lastK])`,
		// value = `{ _k, _v }`.
		s.Diags.Add(DiagRotorNotYetSupported(node, "destructuring `Map<K, V>`"))
		return noneAccessor
	} else if IsDefinitelyType(s, t, IsIterableFunctionLuaTupleType(s)) {
		// iterableFunctionLuaTupleAccessor (L105-117): value = `{ parentId() }`.
		s.Diags.Add(DiagRotorNotYetSupported(node, "destructuring `IterableFunction<LuaTuple<T>>`"))
		return noneAccessor
	} else if IsDefinitelyType(s, t, IsIterableFunctionType(s)) {
		// iterableFunctionAccessor (L119-131): value = `parentId()`.
		s.Diags.Add(DiagRotorNotYetSupported(node, "destructuring `IterableFunction<T>`"))
		return noneAccessor
	} else if IsDefinitelyType(s, t, IsIterableType(s)) {
		s.Diags.Add(DiagNoIterableIteration(node))
		return noneAccessor
	} else if IsDefinitelyType(s, t, IsGeneratorType(s)) ||
		IsDefinitelyType(s, t, IsObjectType) ||
		node.Kind == ast.KindThisKeyword {
		// iterAccessor (L133-141): value = `parentId.next().value`.
		s.Diags.Add(DiagRotorNotYetSupported(node, "destructuring iterator objects"))
		return noneAccessor
	}
	panic("transformer: Destructuring not supported for type: " + s.Checker.TypeToString(t)) // upstream assert(false)
}

// ---------------------------------------------------------------------------
// objectAccessor — util/binding/objectAccessor.ts (L11-36)
// ---------------------------------------------------------------------------

// objectAccessor produces the read expression for one object-pattern
// property. t is the PARENT pattern's type.
//
// QUIRK (port verbatim): computed names get the +1 array adjustment, but
// literal numeric names do NOT — `{ 0: x }` over a tuple emits `parent[0]`
// while `{ [0]: x }` emits `parent[1]` when the parent is array-typed.
func objectAccessor(s *State, parentID luau.AnyIdentifier, t *checker.Type, name *ast.Node) luau.Expression {
	addIndexDiagnostics(s, name, s.GetType(name))

	if ast.IsIdentifier(name) {
		return luau.NewPropertyAccess(parentID, name.Text())
	} else if ast.IsComputedPropertyName(name) {
		return luau.NewComputedIndex(parentID,
			addOneIfArrayType(s, t, TransformExpression(s, name.AsComputedPropertyName().Expression)))
	} else if ast.IsNumericLiteral(name) || ast.IsStringLiteral(name) || ast.IsNoSubstitutionTemplateLiteral(name) {
		return luau.NewComputedIndex(parentID, TransformExpression(s, name))
	} else if ast.IsPrivateIdentifier(name) {
		s.Diags.Add(DiagNoPrivateIdentifier(name))
		return luau.NewNone()
	}
	panic("transformer: objectAccessor unexpected name kind: " + kindName(name.Kind)) // upstream assertNever
}
