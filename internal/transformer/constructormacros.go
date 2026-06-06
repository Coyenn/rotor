package transformer

import (
	"rotor/internal/luau"
	"rotor/tsgo/ast"
)

// This file ports TSTransformer/macros/constructorMacros.ts (COMPLETE). The
// table itself (CONSTRUCTOR_MACROS) lives in macromanager.go.

// wrapWeak ports wrapWeak (L9-14):
// `setmetatable(<macro result>, { __mode = "k" })` — `__mode = "k"` for BOTH
// WeakSet and WeakMap (upstream quirk: weak KEYS only; ported verbatim).
func wrapWeak(s *State, node *ast.Node, macro ConstructorMacro) luau.Expression {
	return luau.NewCall(luau.GlobalID("setmetatable"), luau.NewList[luau.Expression](
		macro(s, node),
		luau.NewMap(luau.NewList(luau.NewMapField(luau.Str("__mode"), luau.Str("k")))),
	))
}

// arrayConstructorMacro ports ArrayConstructor (L16-22): args present ->
// `table.create(<args>)` (`new Array<T>(8)` -> `table.create(8)`; a second
// arg fills); else `{}`.
func arrayConstructorMacro(s *State, node *ast.Node) luau.Expression {
	if arguments := node.AsNewExpression().Arguments; arguments != nil && len(arguments.Nodes) > 0 {
		args := ensureTransformOrder(s, arguments.Nodes)
		return luau.NewCall(luau.GlobalProperty("table", "create"), luau.NewList(args...))
	}
	return luau.NewArray(luau.NewList[luau.Expression]())
}

// arrayLiteralHasSpread reports whether any element of the TS array literal
// is a spread element (upstream `arg.elements.some(ts.isSpreadElement)`).
func arrayLiteralHasSpread(arrayLiteral *ast.Node) bool {
	for _, element := range arrayLiteral.AsArrayLiteralExpression().Elements.Nodes {
		if ast.IsSpreadElement(element) {
			return true
		}
	}
	return false
}

// setConstructorMacro ports SetConstructor (L24-53): no args -> `{}`; a
// spread-free TS ArrayLiteral arg -> set literal (`[v] = true` fields) from
// ensureTransformOrder over its elements; anything else -> `local set = {}`
// plus a prereq `for _, _v in <expr> do set[_v] = true end`, returning the
// temp. NOTE the Set decision is on the TS AST (spreads cause a prereq array,
// which cannot be optimised like the literal path), unlike Map's
// transformed-AST decision.
func setConstructorMacro(s *State, node *ast.Node) luau.Expression {
	arguments := node.AsNewExpression().Arguments
	if arguments == nil || len(arguments.Nodes) == 0 {
		return luau.NewSet(luau.NewList[luau.Expression]())
	}
	arg := arguments.Nodes[0]
	if ast.IsArrayLiteralExpression(arg) && !arrayLiteralHasSpread(arg) {
		elements := ensureTransformOrder(s, arg.AsArrayLiteralExpression().Elements.Nodes)
		return luau.NewSet(luau.NewList(elements...))
	}
	id := s.PushToVar(luau.NewSet(luau.NewList[luau.Expression]()), "set")
	valueID := luau.TempID("v")
	s.Prereq(luau.NewFor(
		luau.NewList[luau.AnyIdentifier](luau.TempID(""), valueID),
		TransformExpression(s, arg),
		luau.NewList[luau.Statement](
			luau.NewAssignment(luau.NewComputedIndex(id, valueID), "=", luau.Bool(true)),
		),
	))
	return id
}

// mapConstructorMacro ports MapConstructor (L55-96): no args -> `{}`; the
// TRANSFORMED arg being a luau Array whose members are ALL Arrays (i.e.
// `[[k, v], ...]` after transform) -> map literal of (head, head.next) pairs;
// anything else -> `local map = {}` plus a prereq
// `for _, _v in <transformed> do map[_v[1]] = _v[2] end`, returning the temp.
// NOTE the decision is on the TRANSFORMED luau AST (spreads/iterables fall to
// the loop path), not the TS AST.
func mapConstructorMacro(s *State, node *ast.Node) luau.Expression {
	arguments := node.AsNewExpression().Arguments
	if arguments == nil || len(arguments.Nodes) == 0 {
		return luau.NewMap(luau.NewList[*luau.MapField]())
	}
	arg := arguments.Nodes[0]
	transformed := TransformExpression(s, arg)
	if arr, ok := transformed.(*luau.Array); ok && arr.Members.Every(func(member luau.Expression) bool {
		_, isArray := member.(*luau.Array)
		return isArray
	}) {
		fields := luau.NewList[*luau.MapField]()
		arr.Members.ForEach(func(member luau.Expression) {
			// each member array always has 2 members, due to map constructor
			// typing (upstream assert + non-null assertion)
			e := member.(*luau.Array)
			if e.Members.Head == nil || e.Members.Head.Next == nil {
				panic("transformer: MapConstructor entry without 2 elements")
			}
			fields.Push(luau.NewMapField(e.Members.Head.Value, e.Members.Head.Next.Value))
		})
		return luau.NewMap(fields)
	}
	id := s.PushToVar(luau.NewMap(luau.NewList[*luau.MapField]()), "map")
	valueID := luau.TempID("v")
	s.Prereq(luau.NewFor(
		luau.NewList[luau.AnyIdentifier](luau.TempID(""), valueID),
		transformed,
		luau.NewList[luau.Statement](
			luau.NewAssignment(
				luau.NewComputedIndex(id, luau.NewComputedIndex(valueID, luau.Num(1))),
				"=",
				luau.NewComputedIndex(valueID, luau.Num(2)),
			),
		),
	))
	return id
}
