package transformer

import (
	"rotor/internal/luau"
	"rotor/tsgo/ast"
)

// This file ports TSTransformer/macros/propertyCallMacros.ts — Phase 3a
// scope: the math-operation region (makeMathMethod/OPERATOR_TO_NAME_MAP/
// makeMathSet, L12-38) plus the math-class rows of PROPERTY_CALL_MACROS
// (L919-928). The container/string method tables (String, Array, Map, Set,
// Promise, ...) land in Phase 3b; those interfaces are declared by
// @rbxts/compiler-types, so the MacroManager's compiler-types fallback
// already detects their methods and raises rotorNotYetSupported.
//
// The math interfaces are different: they are declared by @rbxts/types
// (include/macro_math.d.ts), NOT compiler-types — including the `Number`
// interface row, whose `idiv` member merges into the compiler-types Number
// from macro_math.d.ts. Without real registration the fallback misses them
// and `v.add(w)` silently emits a method call `v:add(w)` instead of `v + w`
// (found by the Phase 3a randomness re-smoke: damage-numbers.ts).

// makeMathMethod ports propertyCallMacros.ts makeMathMethod (L12-20): the
// method call compiles to a Luau binary operation. A non-simple right operand
// gets an explicit ParenthesizedExpression (which also truncates call
// multi-returns); left-operand precedence parens are the renderer's job via
// parent pointers (render/parens.go needsParentheses).
func makeMathMethod(operator luau.BinaryOperator) PropertyCallMacro {
	return func(s *State, node *ast.Node, expression luau.Expression, args []luau.Expression) luau.Expression {
		rhs := args[0]
		if !luau.IsSimple(rhs) {
			rhs = luau.NewParenthesized(rhs)
		}
		return luau.NewBinary(expression, operator, rhs)
	}
}

// operatorToNameMap ports OPERATOR_TO_NAME_MAP (L22-28).
var operatorToNameMap = map[luau.BinaryOperator]string{
	"+":  "add",
	"-":  "sub",
	"*":  "mul",
	"/":  "div",
	"//": "idiv",
}

// makeMathSet ports makeMathSet (L30-38).
func makeMathSet(operators ...luau.BinaryOperator) map[string]PropertyCallMacro {
	result := make(map[string]PropertyCallMacro, len(operators))
	for _, operator := range operators {
		methodName, ok := operatorToNameMap[operator]
		if !ok {
			panic("makeMathSet: no method name for operator " + string(operator)) // upstream assert
		}
		result[methodName] = makeMathMethod(operator)
	}
	return result
}

// propertyCallMacroTable ports the math-class rows of PROPERTY_CALL_MACROS
// (propertyCallMacros.ts L919-928). The remaining rows (String, ArrayLike,
// ReadonlyArray, Array, ReadonlySet, Set, ReadonlyMap, Map, Promise) are
// Phase 3b; their compiler-types-declared methods are already caught by
// GetPropertyCallMacro's fallback.
var propertyCallMacroTable = map[string]map[string]PropertyCallMacro{
	"CFrame":       makeMathSet("+", "-", "*"),
	"UDim":         makeMathSet("+", "-"),
	"UDim2":        makeMathSet("+", "-"),
	"Vector2":      makeMathSet("+", "-", "*", "/", "//"),
	"Vector2int16": makeMathSet("+", "-", "*", "/"),
	"Vector3":      makeMathSet("+", "-", "*", "/", "//"),
	"Vector3int16": makeMathSet("+", "-", "*", "/"),
	"Number":       makeMathSet("//"),
}
