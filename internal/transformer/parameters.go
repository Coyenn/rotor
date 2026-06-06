package transformer

import (
	"rotor/internal/luau"
	"rotor/tsgo/ast"
)

// ---------------------------------------------------------------------------
// Parameters — nodes/transformParameters.ts, nodes/transformInitializer.ts
// ---------------------------------------------------------------------------
//
// Pulled forward from the functions task (Task 3) alongside
// transformFunctionExpression. Binding-pattern parameter names — both the
// `...[a, b]` spread-array-pattern flattening (optimizeArraySpreadParameter,
// L16-51) and ordinary destructuring params (L103-116) — depend on the
// destructuring transforms (Task 4) and raise rotorNotYetSupported until
// they land.

// transformInitializer ports transformInitializer.ts (L6-20): the `= default`
// shape. Default expressions are evaluated lazily inside the nil-check (TS
// semantics: a default is only computed when undefined is passed):
//
//	if id == nil then
//		<init prereqs>
//		id = <init>
//	end
func transformInitializer(s *State, id luau.WritableExpression, initializer *ast.Node) luau.Statement {
	statements := s.CaptureStatements(func() {
		s.Prereq(luau.NewAssignment(id, "=", TransformExpression(s, initializer)))
	})
	return luau.NewIf(luau.NewBinary(id, "==", luau.Nil()), statements, nil)
}

// transformParameters ports transformParameters.ts (L53-124). Returns the
// Luau parameter list, the body-head statements (rest capture, defaults,
// destructuring), and whether the parameter list ends in `...`.
//
//   - methods get an explicit leading `self` parameter (isMethod, CHECKER);
//   - a TS `this` parameter emits nothing (its only effect is via isMethod);
//   - rest `...args` sets hasDotDotDot and prepends `local args = { ... }`;
//   - defaults run BEFORE destructuring, via transformInitializer.
func transformParameters(s *State, node *ast.Node) (parameters *luau.List[luau.AnyIdentifier], statements *luau.List[luau.Statement], hasDotDotDot bool) {
	parameters = luau.NewList[luau.AnyIdentifier]()
	statements = luau.NewList[luau.Statement]()

	if isMethod(s, node) {
		parameters.Push(luau.GlobalID("self"))
	}

	for _, parameterNode := range node.Parameters() {
		parameter := parameterNode.AsParameterDeclaration()
		name := parameter.Name()

		if ast.IsThisIdentifier(name) {
			continue
		}

		if parameter.DotDotDotToken != nil && ast.IsArrayBindingPattern(name) {
			// optimizeArraySpreadParameter (L16-51): `...[a, b]: [A, B]`
			// flattens the pattern elements into real parameters.
			// destructuring task.
			s.Diags.Add(DiagRotorNotYetSupported(name, kindName(name.Kind)))
			continue
		}

		var paramID luau.AnyIdentifier
		if ast.IsIdentifier(name) {
			paramID = TransformIdentifierDefined(s, name)
			ValidateIdentifier(s, name)
		} else {
			paramID = luau.TempID("param")
		}

		if parameter.DotDotDotToken != nil {
			hasDotDotDot = true
			// local args = { ... }
			statements.Push(luau.NewVariableDeclaration(paramID,
				luau.NewArray(luau.NewList[luau.Expression](luau.NewVarArgs()))))
		} else {
			parameters.Push(paramID)
		}

		if parameter.Initializer != nil {
			statements.Push(transformInitializer(s, paramID.(luau.WritableExpression), parameter.Initializer))
		}

		// destructuring
		if !ast.IsIdentifier(name) {
			// transformArrayBindingPattern / transformObjectBindingPattern
			// over paramID: destructuring task.
			s.Diags.Add(DiagRotorNotYetSupported(name, kindName(name.Kind)))
		}
	}

	return parameters, statements, hasDotDotDot
}
