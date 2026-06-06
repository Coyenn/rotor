package transformer

import (
	"rotor/internal/luau"
	"rotor/tsgo/ast"
)

// ---------------------------------------------------------------------------
// Function expressions / arrows —
// expressions/transformFunctionExpression.ts
// ---------------------------------------------------------------------------
//
// Pulled forward from the functions task (Task 3) because the loop
// closure-copy fixture needs closures to actually compile.
// transformFunctionDeclaration and the method transforms land with Task 3.

// transformFunctionExpression ports transformFunctionExpression.ts (L11-47):
// FunctionExpression and ArrowFunction share one transform. Named function
// expressions are banned (name dropped, transform continues). Arrow
// expression bodies reuse the full return transform with prereqs captured
// into the function body — that is the only implicit-return mechanism.
func transformFunctionExpression(s *State, node *ast.Node) luau.Expression {
	if ast.IsFunctionExpression(node) {
		if name := node.AsFunctionExpression().Name(); name != nil {
			s.Diags.Add(DiagNoFunctionExpressionName(name))
		}
	}

	parameters, statements, hasDotDotDot := transformParameters(s, node)

	body := node.Body()
	if ast.IsBlock(body) {
		statements.PushList(TransformStatementList(s, body, body.AsBlock().Statements.Nodes, nil))
	} else {
		var returnStatements *luau.List[luau.Statement]
		prereqs := s.CaptureStatements(func() {
			returnStatements = transformReturnStatementInner(s, body)
		})
		statements.PushList(prereqs)
		statements.PushList(returnStatements)
	}

	isAsync := ast.HasSyntacticModifier(node, ast.ModifierFlagsAsync)

	var asteriskToken *ast.Node
	if ast.IsFunctionExpression(node) {
		asteriskToken = node.AsFunctionExpression().AsteriskToken
	}
	if asteriskToken != nil {
		if isAsync {
			s.Diags.Add(DiagNoAsyncGeneratorFunctions(node))
		}
		// Phase 3: TS.generator — wrapStatementsAsGenerator replaces the body
		// with `return TS.generator(function() <stmts> end)` (runtime lib).
		s.Diags.Add(DiagRotorNotYetSupported(node, "generator functions"))
	} else if isAsync {
		// Phase 3: TS.async — the function value is wrapped
		// `TS.async(<FunctionExpression>)` (runtime lib).
		s.Diags.Add(DiagRotorNotYetSupported(node, "async functions"))
	}

	return luau.NewFunctionExpression(parameters, hasDotDotDot, statements)
}
