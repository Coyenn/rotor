// Package transformer ports roblox-ts's TSTransformer to Go over the tsgo
// compiler frontend.
package transformer

import (
	"fmt"
	"strings"

	"rotor/tsgo/ast"
)

// Diagnostic is the Go port of an upstream diagnostic produced by a factory
// from reference/roblox-ts/src/Shared/diagnostics.ts. Code is the upstream
// factory name (e.g. "noAny"); Message is the exact upstream message text
// (multi-part messages joined with "\n", ANSI color stripped); Node is the
// position source; Warning distinguishes ts.DiagnosticCategory.Warning from
// Error.
type Diagnostic struct {
	Code    string // upstream factory name, e.g. "noAny"
	Message string
	Node    *ast.Node // position source
	Warning bool
}

const repoURL = "https://github.com/roblox-ts/roblox-ts"

// suggestion ports Shared/diagnostics.ts suggestion(): prefixes
// "Suggestion: " (upstream additionally colors the text with kleur.yellow,
// which is presentation-only and not part of the message bytes we keep).
func suggestion(text string) string {
	return "Suggestion: " + text
}

// issue ports Shared/util/createGithubLink.ts issue(): a GitHub issue link
// suffix line (upstream greys the URL with kleur — presentation-only).
func issue(id int) string {
	return fmt.Sprintf("More information: %s/issues/%d", repoURL, id)
}

// errorDiag mirrors upstream error(...messages): joins message parts with
// "\n" into an Error-category diagnostic.
func errorDiag(code string, node *ast.Node, messages ...string) Diagnostic {
	return Diagnostic{Code: code, Message: strings.Join(messages, "\n"), Node: node}
}

// warningDiag mirrors upstream warning(...messages).
func warningDiag(code string, node *ast.Node, messages ...string) Diagnostic {
	return Diagnostic{Code: code, Message: strings.Join(messages, "\n"), Node: node, Warning: true}
}

// ----------------------------------------------------------------------------
// errors — one constructor per upstream factory, in source order of
// reference/roblox-ts/src/Shared/diagnostics.ts. Message text is byte-exact.
// ----------------------------------------------------------------------------

// reserved identifiers

func DiagNoInvalidIdentifier(node *ast.Node) Diagnostic {
	return errorDiag("noInvalidIdentifier", node,
		"Invalid Luau identifier!",
		"Luau identifiers must start with a letter and only contain letters, numbers, and underscores.",
		"Reserved Luau keywords cannot be used as identifiers.",
	)
}

func DiagNoReservedIdentifier(node *ast.Node) Diagnostic {
	return errorDiag("noReservedIdentifier", node, "Cannot use identifier reserved for compiler internal usage.")
}

func DiagNoReservedClassFields(node *ast.Node) Diagnostic {
	return errorDiag("noReservedClassFields", node, "Cannot use class field reserved for compiler internal usage.")
}

func DiagNoClassMetamethods(node *ast.Node) Diagnostic {
	return errorDiag("noClassMetamethods", node, "Metamethods cannot be used in class definitions!")
}

// banned statements

func DiagNoForInStatement(node *ast.Node) Diagnostic {
	return errorDiag("noForInStatement", node, "for-in loop statements are not supported!")
}

func DiagNoLabeledStatement(node *ast.Node) Diagnostic {
	return errorDiag("noLabeledStatement", node, "labels are not supported!")
}

func DiagNoDebuggerStatement(node *ast.Node) Diagnostic {
	return errorDiag("noDebuggerStatement", node, "`debugger` is not supported!")
}

// banned expressions

func DiagNoNullLiteral(node *ast.Node) Diagnostic {
	return errorDiag("noNullLiteral", node, "`null` is not supported!", suggestion("Use `undefined` instead."))
}

func DiagNoPrivateIdentifier(node *ast.Node) Diagnostic {
	return errorDiag("noPrivateIdentifier", node, "Private identifiers are not supported!")
}

func DiagNoTypeOfExpression(node *ast.Node) Diagnostic {
	return errorDiag("noTypeOfExpression", node,
		"`typeof` operator is not supported!",
		suggestion("Use `typeIs(value, type)` or `typeOf(value)` instead."),
	)
}

func DiagNoRegex(node *ast.Node) Diagnostic {
	return errorDiag("noRegex", node, "Regular expressions are not supported!")
}

func DiagNoBigInt(node *ast.Node) Diagnostic {
	return errorDiag("noBigInt", node, "BigInt literals are not supported!")
}

// banned features

func DiagNoAny(node *ast.Node) Diagnostic {
	return errorDiag("noAny", node, "Using values of type `any` is not supported!", suggestion("Use `unknown` instead."))
}

func DiagNoVar(node *ast.Node) Diagnostic {
	return errorDiag("noVar", node, "`var` keyword is not supported!", suggestion("Use `let` or `const` instead."))
}

func DiagNoGetterSetter(node *ast.Node) Diagnostic {
	return errorDiag("noGetterSetter", node, "Getters and Setters are not supported!", issue(457))
}

func DiagNoAutoAccessorModifiers(node *ast.Node) Diagnostic {
	return errorDiag("noAutoAccessorModifiers", node,
		"Getters and Setters are not supported!",
		"The `accessor` keyword requires generating get/set accessors",
		issue(457),
	)
}

func DiagNoEqualsEquals(node *ast.Node) Diagnostic {
	return errorDiag("noEqualsEquals", node, "operator `==` is not supported!", suggestion("Use `===` instead."))
}

func DiagNoExclamationEquals(node *ast.Node) Diagnostic {
	return errorDiag("noExclamationEquals", node, "operator `!=` is not supported!", suggestion("Use `!==` instead."))
}

func DiagNoEnumMerging(node *ast.Node) Diagnostic {
	return errorDiag("noEnumMerging", node, "Enum merging is not supported!")
}

func DiagNoNamespaceMerging(node *ast.Node) Diagnostic {
	return errorDiag("noNamespaceMerging", node, "Namespace merging is not supported!")
}

func DiagNoSpreadDestructuring(node *ast.Node) Diagnostic {
	return errorDiag("noSpreadDestructuring", node, "Operator `...` is not supported for destructuring!")
}

func DiagNoFunctionExpressionName(node *ast.Node) Diagnostic {
	return errorDiag("noFunctionExpressionName", node, "Function expression names are not supported!")
}

func DiagNoPrecedingSpreadElement(node *ast.Node) Diagnostic {
	return errorDiag("noPrecedingSpreadElement", node, "Spread element must come last in a list of arguments!")
}

func DiagNoLuaTupleDestructureAssignmentExpression(node *ast.Node) Diagnostic {
	return errorDiag("noLuaTupleDestructureAssignmentExpression", node,
		"Cannot destructure LuaTuple<T> expression outside of an ExpressionStatement!",
	)
}

func DiagNoExportAssignmentLet(node *ast.Node) Diagnostic {
	return errorDiag("noExportAssignmentLet", node, "Cannot use `export =` on a `let` variable!", suggestion("Use `const` instead."))
}

func DiagNoGlobalThis(node *ast.Node) Diagnostic {
	return errorDiag("noGlobalThis", node, "`globalThis` is not supported!")
}

func DiagNoArguments(node *ast.Node) Diagnostic {
	return errorDiag("noArguments", node, "`arguments` is not supported!")
}

func DiagNoPrototype(node *ast.Node) Diagnostic {
	return errorDiag("noPrototype", node, "`prototype` is not supported!")
}

func DiagNoRobloxSymbolInstanceof(node *ast.Node) Diagnostic {
	// NOTE: upstream's suggestion text is missing the closing backtick and
	// trailing period — preserved byte-exact.
	return errorDiag("noRobloxSymbolInstanceof", node,
		"The `instanceof` operator can only be used on roblox-ts classes!",
		suggestion(`Use `+"`"+`typeIs(myThing, "TypeToCheck") instead`),
	)
}

func DiagNoNonNumberStringRelationOperator(node *ast.Node) Diagnostic {
	return errorDiag("noNonNumberStringRelationOperator", node, "Relation operators can only be used on number or string types!")
}

func DiagNoInstanceMethodCollisions(node *ast.Node) Diagnostic {
	return errorDiag("noInstanceMethodCollisions", node, "Static methods cannot use the same name as instance methods!")
}

func DiagNoStaticMethodCollisions(node *ast.Node) Diagnostic {
	return errorDiag("noStaticMethodCollisions", node, "Instance methods cannot use the same name as static methods!")
}

func DiagNoUnaryPlus(node *ast.Node) Diagnostic {
	return errorDiag("noUnaryPlus", node, "Unary `+` is not supported!", suggestion("Use `tonumber(x)` instead."))
}

func DiagNoNonNumberUnaryMinus(node *ast.Node) Diagnostic {
	return errorDiag("noNonNumberUnaryMinus", node, "Unary `-` is only supported for number types!")
}

func DiagNoAwaitForOf(node *ast.Node) Diagnostic {
	return errorDiag("noAwaitForOf", node, "`await` is not supported in for-of loops!")
}

func DiagNoAsyncGeneratorFunctions(node *ast.Node) Diagnostic {
	return errorDiag("noAsyncGeneratorFunctions", node, "Async generator functions are not supported!")
}

func DiagNoNonStringModuleSpecifier(node *ast.Node) Diagnostic {
	return errorDiag("noNonStringModuleSpecifier", node, "Module specifiers must be a string literal.")
}

func DiagNoIterableIteration(node *ast.Node) Diagnostic {
	return errorDiag("noIterableIteration", node, "Iterating on Iterable<T> is not supported! You must use a more specific type.")
}

func DiagNoMixedTypeCall(node *ast.Node) Diagnostic {
	return errorDiag("noMixedTypeCall", node,
		"Attempted to call a function with mixed types! All definitions must either be a method or a callback.",
	)
}

func DiagNoIndexWithoutCall(node *ast.Node) Diagnostic {
	return errorDiag("noIndexWithoutCall", node,
		"Cannot index a method without calling it!",
		suggestion("Use the form `() => a.b()` instead of `a.b`."),
	)
}

func DiagNoCommentDirectives(node *ast.Node) Diagnostic {
	return errorDiag("noCommentDirectives", node,
		"Usage of `@ts-ignore`, `@ts-expect-error`, and `@ts-nocheck` are not supported!",
		"roblox-ts needs type and symbol info to compile correctly.",
		suggestion("Consider using type assertions or `declare` statements."),
	)
}

// macro methods

func DiagNoOptionalMacroCall(node *ast.Node) Diagnostic {
	return errorDiag("noOptionalMacroCall", node,
		"Macro methods can not be optionally called!",
		suggestion("Macros always exist. Use a normal call."),
	)
}

func DiagNoConstructorMacroWithoutNew(node *ast.Node) Diagnostic {
	return errorDiag("noConstructorMacroWithoutNew", node, "Cannot index a constructor macro without using the `new` operator!")
}

func DiagNoMacroExtends(node *ast.Node) Diagnostic {
	return errorDiag("noMacroExtends", node,
		"Cannot extend from a macro class!",
		suggestion("Store an instance of the macro class in a property."),
	)
}

func DiagNoMacroUnion(node *ast.Node) Diagnostic {
	return errorDiag("noMacroUnion", node, "Macro cannot be applied to a union type!")
}

func DiagNoMacroObjectSpread(node *ast.Node) Diagnostic {
	return errorDiag("noMacroObjectSpread", node,
		"Macro classes cannot be used in an object spread!",
		suggestion("Did you mean to use an array spread? `[ ...exp ]`"),
	)
}

func DiagNoVarArgsMacroSpread(node *ast.Node) Diagnostic {
	return errorDiag("noVarArgsMacroSpread", node, "Macros which use variadic arguments do not support spread expressions!", issue(1149))
}

func DiagNoRangeMacroOutsideForOf(node *ast.Node) Diagnostic {
	return errorDiag("noRangeMacroOutsideForOf", node, "$range() macro is only valid as an expression of a for-of loop!")
}

func DiagNoTupleMacroOutsideReturn(node *ast.Node) Diagnostic {
	return errorDiag("noTupleMacroOutsideReturn", node, "$tuple() macro is only valid as an expression of a return statement!")
}

// import/export

func DiagNoModuleSpecifierFile(node *ast.Node) Diagnostic {
	return errorDiag("noModuleSpecifierFile", node, "Could not find file for import. Did you forget to `npm install`?")
}

func DiagNoInvalidModule(node *ast.Node) Diagnostic {
	return errorDiag("noInvalidModule", node, "You can only use npm scopes that are listed in your typeRoots.")
}

func DiagNoUnscopedModule(node *ast.Node) Diagnostic {
	return errorDiag("noUnscopedModule", node, "You cannot use modules directly under node_modules.")
}

func DiagNoNonModuleImport(node *ast.Node) Diagnostic {
	return errorDiag("noNonModuleImport", node, "Cannot import a non-ModuleScript!")
}

func DiagNoIsolatedImport(node *ast.Node) Diagnostic {
	return errorDiag("noIsolatedImport", node, "Attempted to import a file inside of an isolated container from outside!")
}

func DiagNoServerImport(node *ast.Node) Diagnostic {
	return errorDiag("noServerImport", node,
		"Cannot import a server file from a shared or client location!",
		suggestion("Move the file you want to import to a shared location."),
	)
}

// jsx

func DiagNoPrecedingJsxSpreadElement(node *ast.Node) Diagnostic {
	return errorDiag("noPrecedingJsxSpreadElement", node, "JSX spread expression must come last in children!")
}

// semantic

func DiagExpectedMethodGotFunction(node *ast.Node) Diagnostic {
	return errorDiag("expectedMethodGotFunction", node, "Attempted to assign non-method where method was expected.")
}

func DiagExpectedFunctionGotMethod(node *ast.Node) Diagnostic {
	return errorDiag("expectedFunctionGotMethod", node, "Attempted to assign method where non-method was expected.")
}

// files
//
// Upstream errorWithContext factories: the formatted context lines are
// appended after the static messages; entries that evaluate to `false` are
// filtered out (noRojoData's suggestion when !isPackage).

func DiagNoRojoData(node *ast.Node, path string, isPackage bool) Diagnostic {
	messages := []string{
		fmt.Sprintf("Could not find Rojo data. There is no $path in your Rojo config that covers %s", path),
	}
	if isPackage {
		messages = append(messages, suggestion("Did you forget to add a custom npm scope to your default.project.json?"))
	}
	return errorDiag("noRojoData", node, messages...)
}

func DiagNoPackageImportWithoutScope(node *ast.Node, path string, rbxPath []string) Diagnostic {
	return errorDiag("noPackageImportWithoutScope", node,
		"Imported package Roblox path is missing an npm scope!",
		fmt.Sprintf("Package path: %s", path),
		fmt.Sprintf("Roblox path: %s", strings.Join(rbxPath, ".")),
		suggestion("You might need to update your \"node_modules\" in default.project.json to match:\n"+
			"\"node_modules\": {\n\t\"$className\": \"Folder\",\n\t\"@rbxts\": {\n\t\t\"$path\": \"node_modules/@rbxts\"\n\t}\n}"),
	)
}

// Upstream errorText factories (incorrectFileName, rojoPathInSrc) have no
// node; they surface as project-level text diagnostics, so Node stays nil.

func DiagIncorrectFileName(originalFileName, suggestedFileName, fullPath string) Diagnostic {
	return errorDiag("incorrectFileName", nil,
		fmt.Sprintf("Incorrect file name: `%s`!", originalFileName),
		fmt.Sprintf("Full path: %s", fullPath),
		suggestion(fmt.Sprintf("Change `%s` to `%s`.", originalFileName, suggestedFileName)),
	)
}

func DiagRojoPathInSrc(partitionPath, suggestedPath string) Diagnostic {
	return errorDiag("rojoPathInSrc", nil,
		"Invalid Rojo configuration. $path fields should be relative to out directory.",
		suggestion(fmt.Sprintf("Change the value of $path from %q to %q.", partitionPath, suggestedPath)),
	)
}

// ----------------------------------------------------------------------------
// warnings
// ----------------------------------------------------------------------------

func DiagTruthyChange(node *ast.Node, checksStr string) Diagnostic {
	return warningDiag("truthyChange", node, fmt.Sprintf("Value will be checked against %s", checksStr))
}

func DiagStringOffsetChange(node *ast.Node, text string) Diagnostic {
	return warningDiag("stringOffsetChange", node, fmt.Sprintf("String macros no longer offset inputs: %s", text))
}

// DiagTransformerNotFound ports the warningText factory (no node). err is the
// stringified load error (upstream concatenates `"More info: " + err`).
func DiagTransformerNotFound(name string, err string) Diagnostic {
	return warningDiag("transformerNotFound", nil,
		fmt.Sprintf("Transformer `%s` was not found!", name),
		"More info: "+err,
		suggestion("Did you forget to install the package?"),
	)
}

func DiagRuntimeLibUsedInReplicatedFirst(node *ast.Node) Diagnostic {
	return warningDiag("runtimeLibUsedInReplicatedFirst", node,
		"This statement would generate a call to the runtime library. The runtime library should not be used from ReplicatedFirst.",
	)
}
