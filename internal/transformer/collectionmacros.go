package transformer

import (
	"rotor/internal/luau"
	"rotor/tsgo/ast"
)

// This file ports the Set/Map/Promise region of
// TSTransformer/macros/propertyCallMacros.ts: READONLY_SET_MAP_SHARED_METHODS
// (L736-764), SET_MAP_SHARED_METHODS (L766-808), READONLY_SET_METHODS
// (L810-831), SET_METHODS (L833-853), READONLY_MAP_METHODS (L855-887),
// MAP_METHODS (L889-908), and PROMISE_METHODS (L910-917). The table rows and
// the wrapComments machinery applied to every entry live in
// propertycallmacros.go.
//
// forEach callback arg ORDERS DIFFER between the families (JS signatures):
//   - Set.forEach(cb)  -> cb(value, value, set)  — the value passed TWICE,
//     from a single-id generic-for `for _v in exp do`.
//   - Map.forEach(cb)  -> cb(value, key, map)    — VALUE first, then key,
//     from a two-id generic-for `for _k, _v in exp do`.
//
// WeakSet/WeakMap declare no methods of their own — their calls resolve to
// the Set/Map method symbols through interface inheritance, so only the
// constructors differ (constructormacros.go).

// mergeMacroLists stands in for upstream's object-spread composition
// (`{ ...SHARED, extra }`): later tables override earlier entries.
func mergeMacroLists(lists ...map[string]PropertyCallMacro) map[string]PropertyCallMacro {
	result := make(map[string]PropertyCallMacro)
	for _, list := range lists {
		for name, macro := range list {
			result[name] = macro
		}
	}
	return result
}

// readonlySetMapSharedMethods ports READONLY_SET_MAP_SHARED_METHODS
// (L736-764), spread into both readonly tables.
var readonlySetMapSharedMethods = map[string]PropertyCallMacro{
	// isEmpty (L737): `next(exp) == nil`.
	"isEmpty": func(s *State, node *ast.Node, expression luau.Expression, args []luau.Expression) luau.Expression {
		return luau.NewBinary(
			luau.NewCall(luau.GlobalID("next"), luau.NewList[luau.Expression](expression)),
			"==",
			luau.Nil(),
		)
	},

	// size (L739-755): there is no `#` for hash parts, so count with a loop —
	// `local _size = 0` then a generic-for with ONE valueless tempId
	// (renders `for _ in exp do`) incrementing it.
	"size": func(s *State, node *ast.Node, expression luau.Expression, args []luau.Expression) luau.Expression {
		sizeId := s.PushToVar(luau.Num(0), "size")
		s.Prereq(luau.NewFor(
			luau.NewList[luau.AnyIdentifier](luau.TempID("")),
			expression,
			luau.NewList[luau.Statement](
				luau.NewAssignment(sizeId, "+=", luau.Num(1)),
			),
		))
		return sizeId
	},

	// has (L757-763): `exp[key] ~= nil`.
	"has": func(s *State, node *ast.Node, expression luau.Expression, args []luau.Expression) luau.Expression {
		return luau.NewBinary(
			luau.NewComputedIndex(convertToIndexableExpression(expression), args[0]),
			"~=",
			luau.Nil(),
		)
	},
}

// setMapSharedMethods ports SET_MAP_SHARED_METHODS (L766-808), spread into
// both mutable tables.
var setMapSharedMethods = map[string]PropertyCallMacro{
	// delete (L767-798): the key temp first (pushToVarIfComplex "value");
	// when the boolean result is consumed, the base goes through
	// pushToVarIfNonId (NonId, not IfComplex!) and
	// `local _valueExisted = exp[_value] ~= nil` snapshots existence before
	// the `exp[_value] = nil` clear.
	"delete": func(s *State, node *ast.Node, expression luau.Expression, args []luau.Expression) luau.Expression {
		arg := s.PushToVarIfComplex(args[0], "value")
		valueIsUsed := !isUsedAsStatement(node)
		var valueExistedId *luau.TemporaryIdentifier
		if valueIsUsed {
			expression = s.PushToVarIfNonID(expression, "exp")
			valueExistedId = s.PushToVar(
				luau.NewBinary(
					luau.NewComputedIndex(convertToIndexableExpression(expression), arg),
					"~=",
					luau.Nil(),
				),
				"valueExisted",
			)
		}

		s.Prereq(luau.NewAssignment(
			luau.NewComputedIndex(convertToIndexableExpression(expression), arg),
			"=",
			luau.Nil(),
		))

		if valueIsUsed {
			return valueExistedId
		}
		return luau.NewNone()
	},

	// clear (L800-807): identical to Array.clear — `table.clear(exp)`
	// CallStatement; `nil` when the value is consumed.
	"clear": func(s *State, node *ast.Node, expression luau.Expression, args []luau.Expression) luau.Expression {
		s.Prereq(luau.NewCallStatement(
			luau.NewCall(luau.GlobalProperty("table", "clear"), luau.NewList[luau.Expression](expression)),
		))
		if !isUsedAsStatement(node) {
			return luau.Nil()
		}
		return luau.NewNone()
	},
}

// readonlySetMethods ports READONLY_SET_METHODS (L810-831).
var readonlySetMethods = mergeMacroLists(readonlySetMapSharedMethods, map[string]PropertyCallMacro{
	// forEach (L813-830): single-id generic-for; the callback receives the
	// value TWICE (JS Set.forEach signature `(value, value2, set)`).
	"forEach": func(s *State, node *ast.Node, expression luau.Expression, args []luau.Expression) luau.Expression {
		expression = s.PushToVarIfComplex(expression, "exp")

		callbackId := s.PushToVarIfNonID(args[0], "callback")
		valueId := luau.TempID("v")
		s.Prereq(luau.NewFor(
			luau.NewList[luau.AnyIdentifier](valueId),
			expression,
			luau.NewList[luau.Statement](
				luau.NewCallStatement(
					luau.NewCall(callbackId, luau.NewList[luau.Expression](valueId, valueId, expression)),
				),
			),
		))

		if !isUsedAsStatement(node) {
			return luau.Nil()
		}
		return luau.NewNone()
	},
})

// setMethods ports SET_METHODS (L833-853).
var setMethods = mergeMacroLists(setMapSharedMethods, map[string]PropertyCallMacro{
	// add (L836-852): prereq `exp[value] = true`; returns the SET itself (for
	// chaining) when the value is consumed, so a complex base is temped first.
	"add": func(s *State, node *ast.Node, expression luau.Expression, args []luau.Expression) luau.Expression {
		valueIsUsed := !isUsedAsStatement(node)
		if valueIsUsed {
			expression = s.PushToVarIfComplex(expression, "exp")
		}
		s.Prereq(luau.NewAssignment(
			luau.NewComputedIndex(convertToIndexableExpression(expression), args[0]),
			"=",
			luau.Bool(true),
		))
		if valueIsUsed {
			return expression
		}
		return luau.NewNone()
	},
})

// readonlyMapMethods ports READONLY_MAP_METHODS (L855-887).
var readonlyMapMethods = mergeMacroLists(readonlySetMapSharedMethods, map[string]PropertyCallMacro{
	// forEach (L858-880): two-id generic-for `for _k, _v in exp do`; the
	// callback receives VALUE first (JS Map.forEach signature
	// `(value, key, map)`).
	"forEach": func(s *State, node *ast.Node, expression luau.Expression, args []luau.Expression) luau.Expression {
		expression = s.PushToVarIfComplex(expression, "exp")

		callbackId := s.PushToVarIfNonID(args[0], "callback")
		keyId := luau.TempID("k")
		valueId := luau.TempID("v")
		s.Prereq(luau.NewFor(
			luau.NewList[luau.AnyIdentifier](keyId, valueId),
			expression,
			luau.NewList[luau.Statement](
				luau.NewCallStatement(
					luau.NewCall(callbackId, luau.NewList[luau.Expression](valueId, keyId, expression)),
				),
			),
		))

		if !isUsedAsStatement(node) {
			return luau.Nil()
		}
		return luau.NewNone()
	},

	// get (L882-886): plain `exp[key]` ComputedIndex, no prereqs.
	"get": func(s *State, node *ast.Node, expression luau.Expression, args []luau.Expression) luau.Expression {
		return luau.NewComputedIndex(convertToIndexableExpression(expression), args[0])
	},
})

// mapMethods ports MAP_METHODS (L889-908).
var mapMethods = mergeMacroLists(setMapSharedMethods, map[string]PropertyCallMacro{
	// set (L892-907): like Set.add with `exp[key] = value`; returns the MAP
	// itself (for chaining) when the value is consumed.
	"set": func(s *State, node *ast.Node, expression luau.Expression, args []luau.Expression) luau.Expression {
		keyExp, valueExp := args[0], args[1]
		valueIsUsed := !isUsedAsStatement(node)
		if valueIsUsed {
			expression = s.PushToVarIfComplex(expression, "exp")
		}
		s.Prereq(luau.NewAssignment(
			luau.NewComputedIndex(convertToIndexableExpression(expression), keyExp),
			"=",
			valueExp,
		))
		if valueIsUsed {
			return expression
		}
		return luau.NewNone()
	},
})

// promiseMethods ports PROMISE_METHODS (L910-917): `then` is a Luau keyword
// AND the runtime Promise class names it andThen — `exp:andThen(...)`. No
// prereqs. Everything else on Promise is a REAL method on the runtime
// Promise class and emits as a normal method call.
var promiseMethods = map[string]PropertyCallMacro{
	"then": func(s *State, node *ast.Node, expression luau.Expression, args []luau.Expression) luau.Expression {
		return luau.NewMethodCall("andThen", convertToIndexableExpression(expression), luau.NewList(args...))
	},
}
