package transformer

import (
	"rotor/internal/luau"
	"rotor/tsgo/ast"
)

// ---------------------------------------------------------------------------
// Variable statements — statements/transformVariableStatement.ts
// ---------------------------------------------------------------------------

// transformVariable ports transformVariableStatement.ts transformVariable
// (L19-55) — the identifier-binding core, shared later by for-loop
// initializers and catch clauses.
func transformVariable(s *State, identifier *ast.Node, right luau.Expression) luau.WritableExpression {
	ValidateIdentifier(s, identifier)

	symbol := s.Checker.GetSymbolAtLocation(identifier)
	if symbol == nil {
		panic("transformer: transformVariable: no symbol") // upstream assert
	}

	// export let — declarations of mutable exported symbols write through the
	// exports table instead of creating a local.
	if IsSymbolMutable(s, symbol) {
		if exportAccess := s.GetModuleIDPropertyAccess(symbol); exportAccess != nil {
			if right != nil {
				s.Prereq(luau.NewAssignment(exportAccess, "=", right))
			}
			return exportAccess
		}
	}

	left := TransformIdentifierDefined(s, identifier)

	checkVariableHoist(s, identifier, symbol)
	if s.IsHoisted[symbol] {
		// no need to do `x = nil` if the variable is already created
		if right != nil {
			s.Prereq(luau.NewAssignment(left, "=", right))
		}
	} else {
		var rightValue luau.NodeOrList
		if right != nil {
			rightValue = right
		}
		s.Prereq(luau.NewVariableDeclaration(left, rightValue))
	}

	// Go interface note: luau.AnyIdentifier doesn't embed WritableExpression,
	// but both concrete identifier types implement it.
	return left.(luau.WritableExpression)
}

// checkVariableHoist ports util/checkVariableHoist.ts (L6-39): hoisting for
// variables declared directly inside a switch CaseClause but referenced
// outside their clause, detected upstream with
// ts.FindAllReferences.Core.eachSymbolReferenceInFile. Switch statements are
// outside Phase 2 scope (they raise rotorNotYetSupported before any CaseClause
// declaration can exist), so this is a documented no-op.
// switch-case FindAllReferences hoisting: lands with the switch transform.
func checkVariableHoist(s *State, identifier *ast.Node, symbol *ast.Symbol) {
}

// transformVariableDeclaration ports transformVariableStatement.ts
// transformVariableDeclaration (L101-167) for Identifier names. The
// initializer is transformed BEFORE the name "so references inside of value
// can be hoisted" (upstream comment). Binding patterns (destructuring,
// including the LuaTuple/inline-array optimized paths): later task.
func transformVariableDeclaration(s *State, declaration *ast.Node) *luau.List[luau.Statement] {
	statements := luau.NewList[luau.Statement]()
	decl := declaration.AsVariableDeclaration()

	var value luau.Expression
	if decl.Initializer != nil {
		statements.PushList(s.CaptureStatements(func() {
			value = TransformExpression(s, decl.Initializer)
		}))
	}

	name := decl.Name()
	if ast.IsIdentifier(name) {
		statements.PushList(s.CaptureStatements(func() {
			transformVariable(s, name, value)
		}))
	} else {
		// Array/object binding patterns: destructuring task.
		s.Diags.Add(DiagRotorNotYetSupported(name, kindName(name.Kind)))
	}

	return statements
}

// isVarDeclaration ports transformVariableStatement.ts isVarDeclaration
// (L169-171): neither Const nor Let flag.
func isVarDeclaration(declarationList *ast.Node) bool {
	return declarationList.Flags&ast.NodeFlagsConst == 0 && declarationList.Flags&ast.NodeFlagsLet == 0
}

// transformVariableDeclarationList ports transformVariableStatement.ts
// transformVariableDeclarationList (L173-189): `var` diagnostic, then each
// declaration's prereqs and statements in order.
func transformVariableDeclarationList(s *State, declarationList *ast.Node) *luau.List[luau.Statement] {
	if isVarDeclaration(declarationList) {
		s.Diags.Add(DiagNoVar(declarationList))
	}

	statements := luau.NewList[luau.Statement]()
	for _, declaration := range declarationList.AsVariableDeclarationList().Declarations.Nodes {
		var variableStatements *luau.List[luau.Statement]
		prereqs := s.CaptureStatements(func() {
			variableStatements = transformVariableDeclaration(s, declaration)
		})
		statements.PushList(prereqs)
		statements.PushList(variableStatements)
	}

	return statements
}

// transformVariableStatement ports transformVariableStatement.ts
// transformVariableStatement (L191-196).
func transformVariableStatement(s *State, node *ast.Node) *luau.List[luau.Statement] {
	return transformVariableDeclarationList(s, node.AsVariableStatement().DeclarationList)
}

// ---------------------------------------------------------------------------
// Expression statements — statements/transformExpressionStatement.ts
// ---------------------------------------------------------------------------

// transformExpressionStatement ports transformExpressionStatement (L86-89).
func transformExpressionStatement(s *State, node *ast.Node) *luau.List[luau.Statement] {
	expression := SkipDownwards(node.AsExpressionStatement().Expression)
	return transformExpressionStatementInner(s, expression)
}

// transformExpressionStatementInner ports transformExpressionStatementInner
// (L26-84). Phase 2 first-wave scope:
//   - simple `=` assignment with an identifier lvalue (ported below);
//   - logical assignment (&&=, ||=, ??=), compound operators (+= etc.),
//     property/element lvalues, and the ++/-- statement specialization —
//     full assignment ops: Task 9;
//   - destructuring assignment (array/object literal LHS) — destructuring task;
//   - everything else: wrapExpressionStatement discard semantics.
func transformExpressionStatementInner(s *State, expression *ast.Node) *luau.List[luau.Statement] {
	if ast.IsBinaryExpression(expression) {
		binary := expression.AsBinaryExpression()
		operatorKind := binary.OperatorToken.Kind
		if ast.IsAssignmentOperator(operatorKind) {
			if ast.IsArrayLiteralExpression(binary.Left) || ast.IsObjectLiteralExpression(binary.Left) {
				s.Diags.Add(DiagRotorNotYetSupported(expression, "destructuring assignment"))
				return luau.NewList[luau.Statement]()
			}
			if operatorKind == ast.KindEqualsToken {
				if left := SkipDownwards(binary.Left); ast.IsIdentifier(left) {
					return transformSimpleIdentifierAssignment(s, left, binary.Right)
				}
				// property/element access lvalues: Task 9/10.
				s.Diags.Add(DiagRotorNotYetSupported(expression,
					"assignment to "+kindName(SkipDownwards(binary.Left).Kind)))
				return luau.NewList[luau.Statement]()
			}
			// full assignment ops: Task 9.
			s.Diags.Add(DiagRotorNotYetSupported(expression, "operator `"+kindName(operatorKind)+"`"))
			return luau.NewList[luau.Statement]()
		}
	}
	// ++/-- statement specialization (upstream L76-81): Task 9 — until then
	// prefix/postfix unary falls through to TransformExpression, whose unary
	// path raises rotorNotYetSupported.

	return wrapExpressionStatement(TransformExpression(s, expression))
}

// transformSimpleIdentifierAssignment ports the identifier-lvalue slice of
// transformExpressionStatementInner's assignment branch: upstream routes
// `x = y` through getSimpleAssignmentOperator (always "=" for EqualsToken) and
// transformWritableAssignment(state, left, right, false, false). For an
// identifier lvalue transformWritableExpression degenerates to
// transformExpression(skipDownwards(left)) + a writability assert, the
// readable binding is never consulted (readBeforeWrite=false), and
// getAssignableValue returns the value unchanged for "=".
// full assignment ops: Task 9.
func transformSimpleIdentifierAssignment(s *State, left, right *ast.Node) *luau.List[luau.Statement] {
	transformed := TransformExpression(s, left)
	writable, ok := transformed.(luau.WritableExpression)
	if !ok {
		panic("transformer: assignment lvalue is not writable") // upstream assert
	}

	value, prereqs := s.Capture(func() luau.Expression {
		return TransformExpression(s, right)
	})
	s.PrereqList(prereqs)

	return luau.NewList[luau.Statement](luau.NewAssignment(writable, "=", value))
}

// wrapExpressionStatement ports util/wrapExpressionStatement.ts: an
// expression in statement position is dropped if a temp/none, kept as a call
// statement if a call, otherwise bound to a discarded `local _ = <exp>`.
func wrapExpressionStatement(expression luau.Expression) *luau.List[luau.Statement] {
	switch expression.Kind() {
	case luau.KindTemporaryIdentifier, luau.KindNone:
		return luau.NewList[luau.Statement]()
	}
	if luau.IsCall(expression) {
		return luau.NewList[luau.Statement](luau.NewCallStatement(expression))
	}
	return luau.NewList[luau.Statement](luau.NewVariableDeclaration(luau.TempID(""), expression))
}
