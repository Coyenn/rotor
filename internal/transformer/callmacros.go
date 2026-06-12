package transformer

import (
	"rotor/internal/luau"
	"rotor/tsgo/ast"
)

// This file ports TSTransformer/macros/callMacros.ts (complete) and
// TSTransformer/macros/identifierMacros.ts (complete). The symbol
// registration for both tables lives in macromanager.go (NewMacroManager);
// dispatch happens at transformCallExpressionInner's GetCallMacro hook
// (call.go) and TransformIdentifier's GetIdentifierMacro hook
// (identifier.go). Unlike the property-call macros, these tables are NOT
// comment-wrapped (wrapComments applies only to PROPERTY_CALL_MACROS).

// primitiveLuauTypes ports PRIMITIVE_LUAU_TYPES (callMacros.ts L9-20): the
// strings Luau's `type()` can return — for these, typeIs compiles to the
// cheaper `type(value)` instead of `typeof(value)`.
var primitiveLuauTypes = map[string]bool{
	"nil":      true,
	"boolean":  true,
	"string":   true,
	"number":   true,
	"table":    true,
	"userdata": true,
	"function": true,
	"thread":   true,
	"vector":   true,
	"buffer":   true,
}

// callMacroTable ports CALL_MACROS (callMacros.ts L22-61, 8 entries).
var callMacroTable = map[string]CallMacro{
	// assert(value, message?) (L23-26): the condition gets the full TS
	// truthiness expansion (0/NaN/""/nil guards) before
	// `assert(cond[, message])`. Upstream passes the TS node and lets getType
	// derive the type; rotor's CreateTruthinessChecks takes the type as a
	// parameter.
	"assert": func(s *State, node *ast.Node, expression luau.Expression, args []luau.Expression) luau.Expression {
		arg0 := node.Arguments()[0]
		args[0] = CreateTruthinessChecks(s, args[0], arg0, s.GetType(arg0))
		return luau.NewCall(luau.GlobalID("assert"), luau.NewList(args...))
	},

	// typeOf(...) (L28): `typeof(args...)`.
	"typeOf": func(s *State, node *ast.Node, expression luau.Expression, args []luau.Expression) luau.Expression {
		return luau.NewCall(luau.GlobalID("typeof"), luau.NewList(args...))
	},

	// typeIs(value, typeStr) (L30-37): `type(value) == typeStr` when typeStr
	// is a string literal naming a primitive Luau type, else
	// `typeof(value) == typeStr`.
	"typeIs": func(s *State, node *ast.Node, expression luau.Expression, args []luau.Expression) luau.Expression {
		value, typeStr := args[0], args[1]
		typeFunc := "typeof"
		if str, ok := typeStr.(*luau.StringLiteral); ok && primitiveLuauTypes[str.Value] {
			typeFunc = "type"
		}
		return luau.NewBinary(
			luau.NewCall(luau.GlobalID(typeFunc), luau.NewList[luau.Expression](value)),
			"==",
			typeStr,
		)
	},

	// classIs(instance, className) (L39-42): `value.ClassName == typeStr`.
	"classIs": func(s *State, node *ast.Node, expression luau.Expression, args []luau.Expression) luau.Expression {
		value, typeStr := args[0], args[1]
		return luau.NewBinary(
			luau.NewPropertyAccess(convertToIndexableExpression(value), "ClassName"),
			"==",
			typeStr,
		)
	},

	// identity(v) (L44): compiles to its argument, verbatim.
	"identity": func(s *State, node *ast.Node, expression luau.Expression, args []luau.Expression) luau.Expression {
		return args[0]
	},

	// $range(...) (L46-49): the for-of transform intercepts $range BEFORE the
	// expression transform (forof.go findRangeMacro), so reaching the CALL
	// macro means the call sits outside a for-of expression — an error.
	"$range": func(s *State, node *ast.Node, expression luau.Expression, args []luau.Expression) luau.Expression {
		s.Diags.Add(DiagNoRangeMacroOutsideForOf(node.AsCallExpression().Expression))
		return luau.NewNone()
	},

	// $tuple(...) (L51-54): return statements intercept $tuple BEFORE the
	// expression transform (statements.go isTupleMacro), so reaching the CALL
	// macro means the call sits outside a return expression — an error.
	"$tuple": func(s *State, node *ast.Node, expression luau.Expression, args []luau.Expression) luau.Expression {
		s.Diags.Add(DiagNoTupleMacroOutsideReturn(node))
		return luau.NewNone()
	},

	// $getModuleTree(specifier) (L56-60): converts the flat import-parts
	// array into `{ root, { "rest", "of", "path" } }`. NOTE: like upstream,
	// this reads the RAW TS argument node (node.arguments[0]) — the macro
	// needs the module specifier, not its transformed value.
	// getSourceFileFromModuleSpecifier's resolveModuleName fallback
	// (importexpr.go) covers specifiers of modules never imported normally.
	// rotor extension: FOLDER specifiers (no index.ts) also resolve — see
	// getFolderImportParts.
	"$getModuleTree": func(s *State, node *ast.Node, expression luau.Expression, args []luau.Expression) luau.Expression {
		parts := getModuleTreeImportParts(s, ast.GetSourceFileOfNode(node), node.Arguments()[0])
		rest := luau.NewList[luau.Expression]()
		for _, part := range parts[1:] {
			rest.Push(part)
		}
		return luau.NewArray(luau.NewList[luau.Expression](parts[0], luau.NewArray(rest)))
	},
}

// identifierMacroTable ports macros/identifierMacros.ts IDENTIFIER_MACROS
// (1 entry): a bare `Promise` identifier reads the runtime-lib Promise class
// — `state.TS(node, "Promise")` -> `TS.Promise` (sets UsesRuntimeLib).
var identifierMacroTable = map[string]IdentifierMacro{
	"Promise": func(s *State, node *ast.Node) luau.Expression {
		return s.RuntimeLib(node, "Promise")
	},
}
