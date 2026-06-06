package transformer

import (
	"fmt"

	"rotor/internal/luau"
	"rotor/tsgo/ast"
)

// This file ports the String/ArrayLike region of
// TSTransformer/macros/propertyCallMacros.ts: makeStringCallback (L40-44),
// STRING_CALLBACKS (L46-61), argumentsWithDefaults (L126-157), and
// ARRAY_LIKE_METHODS (L159-161). The table rows and the wrapComments
// machinery applied to every entry live in propertycallmacros.go.
//
// NOTE: only `size` is declared by @rbxts/compiler-types (types/String.d.ts);
// the other 12 String methods are declared by @rbxts/types include/lua.d.ts
// and merge into the same global String interface. Without real registration
// the compiler-types detection fallback misses them and `s.gsub(...)`
// silently emits a method call `s:gsub(...)` instead of upstream's
// `string.gsub(s, ...)` — the same silent-wrong-output class as the Phase 3a
// math macros (damage-numbers.ts bug).

// makeStringCallback ports makeStringCallback (L40-44): the method call
// compiles to `string.X(expression, ...args)` — plain call, the base
// expression FIRST, the transformed call args appended verbatim. The macro
// does no index offsetting: the Luau string API is 1-based and the rbxts
// String declarations expose the raw API. Upstream passes the shared
// luau.globals.string.X node; rotor constructs the property access per
// invocation (identical bytes; avoids mutating a package-level node's parent
// pointer on adoption).
func makeStringCallback(name string) PropertyCallMacro {
	return func(s *State, node *ast.Node, expression luau.Expression, args []luau.Expression) luau.Expression {
		return luau.NewCall(
			luau.GlobalProperty("string", name),
			luau.NewList(append([]luau.Expression{expression}, args...)...),
		)
	}
}

// sizeMacro is the shared `size` emit: `#expression` (`luau.unary("#", exp)`).
// STRING_CALLBACKS.size (L47) and ARRAY_LIKE_METHODS.size (L160) are
// textually identical upstream lambdas.
func sizeMacro(s *State, node *ast.Node, expression luau.Expression, args []luau.Expression) luau.Expression {
	return luau.NewUnary("#", expression)
}

// stringCallbacks ports STRING_CALLBACKS (L46-61). None of these macros
// produce prereqs, so the wrapComments markers never appear for String
// methods. `find`/`gsub`/`match`/`byte` return LuaTuple — runCallMacro's
// wrapReturnIfLuaTuple packs the call in `{ ... }` unless the syntactic
// context consumes the multiple values.
var stringCallbacks = map[string]PropertyCallMacro{
	"size": sizeMacro,

	"byte":    makeStringCallback("byte"),
	"find":    makeStringCallback("find"),
	"format":  makeStringCallback("format"),
	"gmatch":  makeStringCallback("gmatch"),
	"gsub":    makeStringCallback("gsub"),
	"lower":   makeStringCallback("lower"),
	"match":   makeStringCallback("match"),
	"rep":     makeStringCallback("rep"),
	"reverse": makeStringCallback("reverse"),
	"split":   makeStringCallback("split"),
	"sub":     makeStringCallback("sub"),
	"upper":   makeStringCallback("upper"),
}

// arrayLikeMethods ports ARRAY_LIKE_METHODS (L159-161). Registered for the
// ArrayLike interface (compiler-types types/Array.d.ts), whose `size` method
// symbol ReadonlyArray/Array reach through interface inheritance. String.size
// and the Set/Map size entries (Task 5) are separate registrations on their
// own interfaces' method symbols.
var arrayLikeMethods = map[string]PropertyCallMacro{
	"size": sizeMacro,
}

// argumentsWithDefaults ports argumentsWithDefaults (L126-157): fill omitted
// optional args with the macro's declared defaults. For each PROVIDED arg
// that is not a simple primitive: push to var (hint valueToIdStr(arg) ||
// "arg<i>") and prereq `if argN == nil then argN = defaults[i] end`; each
// MISSING trailing arg becomes defaults[j] literally. Currently used only by
// ReadonlyArray.join (Task 4).
func argumentsWithDefaults(s *State, args []luau.Expression, defaults []luau.Expression) []luau.Expression {
	// potentially nil arguments
	for i := range args {
		if !luau.IsSimplePrimitive(args[i]) {
			name := ValueToIdStr(args[i])
			if name == "" {
				name = fmt.Sprintf("arg%d", i)
			}
			temp := s.PushToVar(args[i], name)
			args[i] = temp
			s.Prereq(luau.NewIf(
				luau.NewBinary(temp, "==", luau.Nil()),
				luau.NewList[luau.Statement](luau.NewAssignment(temp, "=", defaults[i])),
				nil,
			))
		}
	}

	// not specified
	for j := len(args); j < len(defaults); j++ {
		args = append(args, defaults[j])
	}

	return args
}
