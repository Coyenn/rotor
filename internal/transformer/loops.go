package transformer

import (
	"math"

	"rotor/internal/luau"
	"rotor/tsgo/ast"
	"rotor/tsgo/checker"
	"rotor/tsgo/jsnum"
)

// ---------------------------------------------------------------------------
// While statements — statements/transformWhileStatement.ts
// ---------------------------------------------------------------------------

// transformWhileStatement ports transformWhileStatement.ts (L9-37). A
// condition whose transform produced prereqs must be re-evaluated every
// iteration, so the prereqs move into the loop body followed by
// `if not cond then break end`, and the while condition becomes `true`.
func transformWhileStatement(s *State, node *ast.Node) *luau.List[luau.Statement] {
	statement := node.AsWhileStatement()
	whileStatements := luau.NewList[luau.Statement]()

	conditionExp, conditionPrereqs := s.Capture(func() luau.Expression {
		return CreateTruthinessChecks(s,
			TransformExpression(s, statement.Expression), statement.Expression, s.GetType(statement.Expression))
	})

	if !conditionPrereqs.IsEmpty() {
		whileStatements.PushList(conditionPrereqs)
		whileStatements.Push(luau.NewIf(
			luau.NewUnary("not", conditionExp),
			luau.NewList[luau.Statement](luau.NewBreak()),
			nil,
		))
		conditionExp = luau.Bool(true)
	}

	whileStatements.PushList(TransformStatementList(s, statement.Statement, getStatements(statement.Statement), nil))

	return luau.NewList[luau.Statement](luau.NewWhile(conditionExp, whileStatements))
}

// ---------------------------------------------------------------------------
// Do statements — statements/transformDoStatement.ts (do/while)
// ---------------------------------------------------------------------------

// transformDoStatement ports transformDoStatement.ts (L9-37). The body is
// wrapped in an inner `do ... end` so its locals cannot collide with the
// condition (repeat..until conditions can see body locals in Luau). Inversion
// micro-opt: `do {} while (!x)` strips the `!` and skips the `not` wrap,
// since repeat..until exits when the condition is true while do/while
// continues when true (the two inversions cancel).
func transformDoStatement(s *State, node *ast.Node) *luau.List[luau.Statement] {
	doStatement := node.AsDoStatement()
	expression := doStatement.Expression
	statements := TransformStatementList(s, doStatement.Statement, getStatements(doStatement.Statement), nil)

	conditionIsInvertedInLuau := true
	if ast.IsPrefixUnaryExpression(expression) && expression.AsPrefixUnaryExpression().Operator == ast.KindExclamationToken {
		expression = expression.AsPrefixUnaryExpression().Operand
		conditionIsInvertedInLuau = false
	}

	condition, conditionPrereqs := s.Capture(func() luau.Expression {
		return CreateTruthinessChecks(s, TransformExpression(s, expression), expression, s.GetType(expression))
	})

	repeatStatements := luau.NewList[luau.Statement]()
	repeatStatements.Push(luau.NewDo(statements))
	repeatStatements.PushList(conditionPrereqs)

	if conditionIsInvertedInLuau {
		condition = luau.NewUnary("not", condition)
	}

	return luau.NewList[luau.Statement](luau.NewRepeat(condition, repeatStatements))
}

// ---------------------------------------------------------------------------
// C-style for statements — statements/transformForStatement.ts
// ---------------------------------------------------------------------------

// panicOnLoopClosureCapture is the Phase 2 stand-in for the fallback path's
// per-iteration let-capture copy machinery (transformForStatement.ts
// isIdWriteOrAsyncRead L78-100 / canSkipClone L73-76, both built on
// ts.FindAllReferences): upstream emits `local _i = ...` outer copies,
// per-iteration `local i = _i` headers and `_i = i` finalizers (cloned before
// every `continue` via addFinalizers L35-71) so closures capture each
// iteration's binding. Function transforms have not landed, so any loop whose
// closure would need that machinery cannot compile correctly — fail loudly.
//
// Conservative trigger (documented simplification): any function-like node
// anywhere inside the for statement that references a loop-declared symbol.
// Upstream's other copy trigger — a WRITE to the loop variable outside the
// incrementor with no closure involved — is deliberately NOT reproduced: the
// copies are semantically inert without capture (byte divergence from rbxtsc
// for such loops is a known Phase 3 gap, never wrong behavior).
func panicOnLoopClosureCapture(s *State, forStatementNode *ast.Node, declarationList *ast.Node) {
	symbols := map[*ast.Symbol]struct{}{}
	for _, declaration := range declarationList.AsVariableDeclarationList().Declarations.Nodes {
		name := declaration.AsVariableDeclaration().Name()
		// Binding patterns raise rotorNotYetSupported in
		// transformVariableDeclaration before any capture could matter.
		if ast.IsIdentifier(name) {
			if symbol := s.Checker.GetSymbolAtLocation(name); symbol != nil {
				symbols[symbol] = struct{}{}
			}
		}
	}
	if len(symbols) == 0 {
		return
	}

	var referencesLoopVar func(node *ast.Node) bool
	referencesLoopVar = func(node *ast.Node) bool {
		if ast.IsIdentifier(node) {
			if symbol := s.Checker.GetSymbolAtLocation(node); symbol != nil {
				if _, ok := symbols[symbol]; ok {
					return true
				}
			}
		}
		return node.ForEachChild(referencesLoopVar)
	}
	var visit func(node *ast.Node) bool
	visit = func(node *ast.Node) bool {
		if ast.IsFunctionLike(node) {
			return referencesLoopVar(node)
		}
		return node.ForEachChild(visit)
	}
	if forStatementNode.ForEachChild(visit) {
		panic("rotor: loop closure capture not yet supported (phase 3)")
	}
}

// transformForStatementFallback ports transformForStatementFallback
// (L102-297), the fully general while-loop desugaring, minus the
// per-iteration let-capture copies (see panicOnLoopClosureCapture). With the
// copies gone the finalizer list is always empty, so addFinalizers (L35-71,
// the continue-statement writeback cloning) and the finalizer tail-push
// (L278-284) are deferred to Phase 3 alongside them.
func transformForStatementFallback(s *State, node *ast.Node) *luau.List[luau.Statement] {
	forStatement := node.AsForStatement()
	initializer, condition, incrementor := forStatement.Initializer, forStatement.Condition, forStatement.Incrementor
	statement := forStatement.Statement

	result := luau.NewList[luau.Statement]()
	whileStatements := luau.NewList[luau.Statement]()

	if initializer != nil {
		if ast.IsVariableDeclarationList(initializer) {
			if isVarDeclaration(initializer) {
				s.Diags.Add(DiagNoVar(node))
			}

			panicOnLoopClosureCapture(s, node, initializer)

			// transformVariableDeclaration per declaration (L145-157): each
			// declaration's prereqs land before its statements.
			for _, declaration := range initializer.AsVariableDeclarationList().Declarations.Nodes {
				var decStatements *luau.List[luau.Statement]
				decPrereqs := s.CaptureStatements(func() {
					decStatements = transformVariableDeclaration(s, declaration)
				})
				result.PushList(decPrereqs)
				result.PushList(decStatements)
			}
		} else {
			// Expression initializer (L204-208).
			var statements *luau.List[luau.Statement]
			prereqs := s.CaptureStatements(func() {
				statements = transformExpressionStatementInner(s, initializer)
			})
			result.PushList(prereqs)
			result.PushList(statements)
		}
	}

	// Incrementor (L211-247): guarded so it runs at the TOP of every
	// iteration except the first — `continue` still triggers the increment,
	// since Luau `continue` jumps to the loop top.
	if incrementor != nil {
		shouldIncrement := luau.TempID("shouldIncrement")

		// local _shouldIncrement = false
		result.Push(luau.NewVariableDeclaration(shouldIncrement, luau.Bool(false)))

		incrementorStatements := luau.NewList[luau.Statement]()
		var statements *luau.List[luau.Statement]
		prereqs := s.CaptureStatements(func() {
			statements = transformExpressionStatementInner(s, incrementor)
		})
		incrementorStatements.PushList(prereqs)
		incrementorStatements.PushList(statements)

		// if _shouldIncrement then
		// 	[incrementorStatements]
		// else
		// 	_shouldIncrement = true
		// end
		whileStatements.Push(luau.NewIf(
			shouldIncrement,
			incrementorStatements,
			luau.NewList[luau.Statement](luau.NewAssignment(shouldIncrement, "=", luau.Bool(true))),
		))
	}

	// Condition (L249-274): if ANY whileStatements precede the body
	// (incrementor guard or condition prereqs), the condition must be
	// evaluated after them — `if not cond then break end` and the while
	// condition becomes `true`.
	conditionExp, conditionPrereqs := s.Capture(func() luau.Expression {
		if condition != nil {
			return CreateTruthinessChecks(s, TransformExpression(s, condition), condition, s.GetType(condition))
		}
		return luau.Bool(true)
	})

	whileStatements.PushList(conditionPrereqs)

	if !whileStatements.IsEmpty() {
		if condition != nil {
			// if not [conditionExp] then
			//	break
			// end
			whileStatements.Push(luau.NewIf(
				luau.NewUnary("not", conditionExp),
				luau.NewList[luau.Statement](luau.NewBreak()),
				nil,
			))
		}
		conditionExp = luau.Bool(true)
	}

	whileStatements.PushList(TransformStatementList(s, statement, getStatements(statement), nil))

	result.Push(luau.NewWhile(conditionExp, whileStatements))

	// Assembly (L286-296): multiple statements wrap in `do ... end` to scope
	// the loop variable declarations.
	if result.Head == result.Tail {
		return result
	}
	return luau.NewList[luau.Statement](luau.NewDo(result))
}

// getOptimizedIncrementorStepValue ports getOptimizedIncrementorStepValue
// (L299-332): `i += intLit` / `i -= intLit` / `i++` / `i--` yield the step
// value; anything else disqualifies the optimization. Upstream quirk ported
// faithfully: the `-=` branch (L309-315) never checks that the left side is
// the loop variable, unlike the `+=` branch.
func getOptimizedIncrementorStepValue(s *State, incrementor *ast.Node, idSymbol *ast.Symbol) (float64, bool) {
	if ast.IsBinaryExpression(incrementor) {
		binary := incrementor.AsBinaryExpression()
		if ast.IsIdentifier(binary.Left) &&
			s.Checker.GetSymbolAtLocation(binary.Left) == idSymbol &&
			binary.OperatorToken.Kind == ast.KindPlusEqualsToken &&
			ast.IsNumericLiteral(binary.Right) &&
			isProbablyInteger(s, binary.Right) {
			value, err := luau.JSNumberParse(getText(s, binary.Right))
			return value, err == nil
		} else if binary.OperatorToken.Kind == ast.KindMinusEqualsToken &&
			ast.IsNumericLiteral(binary.Right) &&
			isProbablyInteger(s, binary.Right) {
			value, err := luau.JSNumberParse(getText(s, binary.Right))
			return -value, err == nil
		}
	} else if ast.IsPostfixUnaryExpression(incrementor) || ast.IsPrefixUnaryExpression(incrementor) {
		operand, operator := unaryOperandAndOperator(incrementor)
		if ast.IsIdentifier(operand) && s.Checker.GetSymbolAtLocation(operand) == idSymbol {
			if operator == ast.KindPlusPlusToken {
				return 1, true
			} else if operator == ast.KindMinusMinusToken {
				return -1, true
			}
		}
	}
	return 0, false
}

// unaryOperandAndOperator extracts the operand/operator pair from either
// unary expression flavor.
func unaryOperandAndOperator(node *ast.Node) (*ast.Node, ast.Kind) {
	if ast.IsPrefixUnaryExpression(node) {
		unary := node.AsPrefixUnaryExpression()
		return unary.Operand, unary.Operator
	}
	unary := node.AsPostfixUnaryExpression()
	return unary.Operand, unary.Operator
}

// isSizeMacro ports isSizeMacro (L334-346): a call whose callee symbol is the
// `size` property-call macro (e.g. `arr.size()`), using the Phase 2 macro
// stand-in from call.go.
func isSizeMacro(s *State, expression *ast.Node) bool {
	if ast.IsCallExpression(expression) {
		expType := s.Checker.GetNonOptionalType(s.GetType(expression.AsCallExpression().Expression))
		symbol := GetFirstDefinedSymbol(s, expType)
		if symbol != nil && symbol.Name == "size" && isPropertyCallMacroSymbol(symbol) {
			return true
		}
	}
	return false
}

// isUnaryExpressionWithWrite ports ts.isUnaryExpressionWithWrite: postfix
// unary (always ++/--) or prefix unary with ++/--.
func isUnaryExpressionWithWrite(node *ast.Node) bool {
	switch node.Kind {
	case ast.KindPostfixUnaryExpression:
		return true
	case ast.KindPrefixUnaryExpression:
		operator := node.AsPrefixUnaryExpression().Operator
		return operator == ast.KindPlusPlusToken || operator == ast.KindMinusMinusToken
	}
	return false
}

// isMutatedInBody ports isMutatedInBody (L348-366): true when any reference
// to the loop variable inside the body is an assignment target or a ++/--
// operand. Upstream walks references with ts.FindAllReferences; rotor walks
// the body resolving each identifier's symbol — equivalent within one file.
func isMutatedInBody(s *State, idSymbol *ast.Symbol, body *ast.Node) bool {
	var visit func(node *ast.Node) bool
	visit = func(node *ast.Node) bool {
		if ast.IsIdentifier(node) && s.Checker.GetSymbolAtLocation(node) == idSymbol {
			parent := SkipUpwards(node).Parent
			if parent != nil {
				if ast.IsAssignmentExpression(parent, false) && SkipDownwards(parent.AsBinaryExpression().Left) == node {
					return true
				}
				if isUnaryExpressionWithWrite(parent) {
					operand, _ := unaryOperandAndOperator(parent)
					if SkipDownwards(operand) == node {
						return true
					}
				}
			}
			return false
		}
		return node.ForEachChild(visit)
	}
	return visit(body)
}

// isProbablyInteger ports isProbablyInteger (L368-390): integer numeric
// literal; `+ - * **` of two such; unary ± of one; a `.size()` macro call; or
// a checker type that is an integer number literal. NOTE the upstream chain
// shape: a binary/prefix-unary expression with a non-matching operator falls
// straight to false without consulting the checker.
func isProbablyInteger(s *State, expression *ast.Node) bool {
	if ast.IsNumericLiteral(expression) {
		value, err := luau.JSNumberParse(getText(s, expression))
		return err == nil && !math.IsInf(value, 0) && value == math.Trunc(value)
	} else if ast.IsBinaryExpression(expression) {
		binary := expression.AsBinaryExpression()
		switch binary.OperatorToken.Kind {
		case ast.KindPlusToken, ast.KindMinusToken, ast.KindAsteriskToken, ast.KindAsteriskAsteriskToken:
			return isProbablyInteger(s, binary.Left) && isProbablyInteger(s, binary.Right)
		}
	} else if ast.IsPrefixUnaryExpression(expression) {
		unary := expression.AsPrefixUnaryExpression()
		if unary.Operator == ast.KindPlusToken || unary.Operator == ast.KindMinusToken {
			return isProbablyInteger(s, unary.Operand)
		}
	} else if isSizeMacro(s, expression) {
		return true
	} else if IsDefinitelyType(s, s.GetType(expression), isIntegerLiteralTypeCheck) {
		return true
	}
	return false
}

// isIntegerLiteralTypeCheck ports the L386 callback:
// `t.isNumberLiteral() && Number.isInteger(t.value)`.
var isIntegerLiteralTypeCheck = TypeCheck{check: func(t *checker.Type) bool {
	if !t.IsNumberLiteral() {
		return false
	}
	value, ok := t.AsLiteralType().Value().(jsnum.Number)
	if !ok {
		return false
	}
	f := float64(value)
	return !math.IsNaN(f) && !math.IsInf(f, 0) && f == math.Trunc(f)
}}

// transformForStatementOptimized ports transformForStatementOptimized
// (L392-489): `for (let i = a; i < b; i += s)` (and <=, >, >= variants)
// becomes Luau `for i = a, b±1, s do`. Returns nil when any precondition
// fails: single identifier declaration with an isProbablyInteger initializer;
// incrementor with an extractable integer step; condition operator direction
// matching the step sign (`<`/`<=` need step >= 0, `>`/`>=` need step <= 0;
// `!==` etc. never optimize); isProbablyInteger condition RHS; loop variable
// not mutated in the body. Emitted bounds: `<` -> offset(end, -1), `>` ->
// offset(end, +1) — both constant-fold through offsetExpr when the bound is a
// literal (`i < 10` -> `9`) and stay symbolic otherwise (`i < limit` ->
// `limit - 1`); `<=`/`>=` use the bound as-is.
func transformForStatementOptimized(s *State, node *ast.Node) *luau.List[luau.Statement] {
	forStatement := node.AsForStatement()
	initializer, condition, incrementor := forStatement.Initializer, forStatement.Condition, forStatement.Incrementor
	statement := forStatement.Statement

	// validate initializer exists and is a single identifier `x` with a value
	// that is _probably_ an integer

	if initializer == nil || !ast.IsVariableDeclarationList(initializer) ||
		len(initializer.AsVariableDeclarationList().Declarations.Nodes) != 1 {
		return nil
	}

	declaration := initializer.AsVariableDeclarationList().Declarations.Nodes[0].AsVariableDeclaration()
	decName, decInit := declaration.Name(), declaration.Initializer
	if !ast.IsIdentifier(decName) || decInit == nil {
		return nil
	}

	idSymbol := s.Checker.GetSymbolAtLocation(decName)
	if idSymbol == nil {
		return nil
	}

	if !isProbablyInteger(s, decInit) {
		return nil
	}

	// validate incrementor exists and is _probably_ an integer change in `x`

	if incrementor == nil {
		return nil
	}

	stepValue, ok := getOptimizedIncrementorStepValue(s, incrementor, idSymbol)
	if !ok {
		return nil
	}

	// validate condition exists and is a BinaryExpression with an operator
	// that matches the incrementor

	if condition == nil || !ast.IsBinaryExpression(condition) {
		return nil
	}
	conditionBinary := condition.AsBinaryExpression()
	operatorKind := conditionBinary.OperatorToken.Kind

	switch operatorKind {
	case ast.KindLessThanToken, ast.KindLessThanEqualsToken:
		// do not optimize for cases which should never run like:
		// for (let i = 10; i < 0; i--)
		if stepValue < 0 {
			return nil
		}
	case ast.KindGreaterThanToken, ast.KindGreaterThanEqualsToken:
		// do not optimize for cases which should never run like:
		// for (let i = 0; i > 10; i++)
		if stepValue > 0 {
			return nil
		}
	default:
		// do not optimize for other comparison operators like !==, ===
		return nil
	}

	if !isProbablyInteger(s, conditionBinary.Right) {
		return nil
	}

	if isMutatedInBody(s, idSymbol, statement) {
		return nil
	}

	// commit to the optimization and start transforming..

	result := luau.NewList[luau.Statement]()

	id := TransformIdentifierDefined(s, decName)

	start, startPrereqs := s.Capture(func() luau.Expression { return TransformExpression(s, decInit) })
	result.PushList(startPrereqs)

	end, endPrereqs := s.Capture(func() luau.Expression { return TransformExpression(s, conditionBinary.Right) })
	result.PushList(endPrereqs)

	step := luau.Num(stepValue)
	statements := TransformStatementList(s, statement, getStatements(statement), nil)

	if operatorKind == ast.KindLessThanToken {
		end = offsetExpr(end, -1)
	} else if operatorKind == ast.KindGreaterThanToken {
		end = offsetExpr(end, 1)
	}

	result.Push(luau.NewNumericFor(id, start, end, step, statements))

	return result
}

// transformForStatement ports transformForStatement (L491-499). Upstream
// gates the optimized pass on `projectOptions.optimizedLoops` (default true);
// rotor has no project-options surface yet, so the pass is always on.
func transformForStatement(s *State, node *ast.Node) *luau.List[luau.Statement] {
	if optimized := transformForStatementOptimized(s, node); optimized != nil {
		return optimized
	}
	return transformForStatementFallback(s, node)
}
