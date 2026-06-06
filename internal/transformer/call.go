package transformer

import (
	"rotor/internal/luau"
	"rotor/tsgo/ast"
	"rotor/tsgo/checker"
)

// This file ports expressions/transformCallExpression.ts,
// nodes/transformOptionalChain.ts (non-optional fold only), and their utils:
// util/convertToIndexableExpression.ts, util/expressionMightMutate.ts,
// util/wrapReturnIfLuaTuple.ts, util/arrayBindingPatternContainsHoists.ts.
//
// Macro hook points query the MacroManager (macromanager.go); registered
// entries with a nil Macro are known upstream macros rotor has not
// implemented yet and raise rotorNotYetSupported instead of silently-wrong
// output. full macro tables (runCallMacro): Phase 3b.

// convertToIndexableExpression ports util/convertToIndexableExpression.ts:
// wrap non-indexable expressions in parentheses so they can be indexed or
// called.
func convertToIndexableExpression(expression luau.Expression) luau.IndexableExpression {
	if luau.IsIndexableExpression(expression) {
		return expression.(luau.IndexableExpression)
	}
	return luau.NewParenthesized(expression)
}

// ---------------------------------------------------------------------------
// expressionMightMutate — util/expressionMightMutate.ts (COMPLETE)
// ---------------------------------------------------------------------------

// expressionMightMutate reports whether the rendered luau expression could
// change value if statements run between its computation and its use. node is
// the originating TS expression (optional): reads of const identifiers cannot
// change.
func expressionMightMutate(s *State, expression luau.Expression, node *ast.Node) bool {
	switch e := expression.(type) {
	case *luau.TemporaryIdentifier:
		// Assume tempIds are never re-assigned after being returned
		return false
	case *luau.ParenthesizedExpression:
		return expressionMightMutate(s, e.Expression, nil)
	case *luau.FunctionExpression:
		return false
	case *luau.VarArgsLiteral:
		return false
	case *luau.IfExpression:
		return expressionMightMutate(s, e.Condition, nil) ||
			expressionMightMutate(s, e.Expression, nil) ||
			expressionMightMutate(s, e.Alternative, nil)
	case *luau.BinaryExpression:
		return expressionMightMutate(s, e.Left, nil) || expressionMightMutate(s, e.Right, nil)
	case *luau.UnaryExpression:
		return expressionMightMutate(s, e.Expression, nil)
	case *luau.Array:
		return e.Members.Some(func(member luau.Expression) bool { return expressionMightMutate(s, member, nil) })
	case *luau.Set:
		return e.Members.Some(func(member luau.Expression) bool { return expressionMightMutate(s, member, nil) })
	case *luau.Map:
		return e.Fields.Some(func(field *luau.MapField) bool {
			return expressionMightMutate(s, field.Index, nil) || expressionMightMutate(s, field.Value, nil)
		})
	}

	if luau.IsSimplePrimitive(expression) {
		return false
	}

	if node != nil {
		node = SkipDownwards(node)
		if ast.IsIdentifier(node) {
			if symbol := s.Checker.GetSymbolAtLocation(node); symbol != nil && !IsSymbolMutable(s, symbol) {
				return false
			}
		}
	}

	// Identifier / ComputedIndexExpression / PropertyAccessExpression /
	// CallExpression / MethodCallExpression
	return true
}

// ---------------------------------------------------------------------------
// wrapReturnIfLuaTuple — util/wrapReturnIfLuaTuple.ts
// ---------------------------------------------------------------------------

// arrayBindingPatternContainsHoists ports
// util/arrayBindingPatternContainsHoists.ts: does any identifier bound by the
// pattern require hoisting?
func arrayBindingPatternContainsHoists(s *State, arrayBindingPattern *ast.Node) bool {
	for _, element := range arrayBindingPattern.AsBindingPattern().Elements.Nodes {
		// If it's not a BindingElement, it must be an OmittedExpression — no variable.
		// Nested binding patterns get temp ids; their hoisting is handled elsewhere.
		if ast.IsBindingElement(element) {
			name := element.Name()
			if name != nil && ast.IsIdentifier(name) {
				if symbol := s.Checker.GetSymbolAtLocation(name); symbol != nil {
					// isHoisted is marked inside checkVariableHoist
					checkVariableHoist(s, name, symbol)
					if s.IsHoisted[symbol] {
						return true
					}
				}
			}
		}
	}
	return false
}

// shouldWrapLuaTuple ports wrapReturnIfLuaTuple.ts shouldWrapLuaTuple
// (L8-56): a LuaTuple-typed call is packed `{ f() }` UNLESS the syntactic
// context consumes the multiple values.
func shouldWrapLuaTuple(s *State, node *ast.Node, exp luau.Expression) bool {
	if !luau.IsCall(exp) {
		return true
	}

	child := SkipUpwards(node)
	parent := child.Parent
	if parent == nil {
		return true
	}

	// `foo();`
	if ast.IsExpressionStatement(parent) {
		return false
	}

	// if part of for statement definition, except if used as the condition
	if ast.IsForStatement(parent) && parent.AsForStatement().Condition != child {
		return false
	}

	// `const [a] = foo()`
	if ast.IsVariableDeclaration(parent) {
		name := parent.AsVariableDeclaration().Name()
		if name != nil && ast.IsArrayBindingPattern(name) && !arrayBindingPatternContainsHoists(s, name) {
			return false
		}
	}

	// `[a] = foo()`
	if ast.IsAssignmentExpression(parent, false) && ast.IsArrayLiteralExpression(parent.AsBinaryExpression().Left) {
		return false
	}

	// `foo()[n]`
	if ast.IsElementAccessExpression(parent) {
		return false
	}

	// `return foo()`
	if ast.IsReturnStatement(parent) {
		return false
	}

	// `void foo()`
	if ast.IsVoidExpression(parent) {
		return false
	}

	return true
}

// wrapReturnIfLuaTuple ports wrapReturnIfLuaTuple.ts (L58-63).
func wrapReturnIfLuaTuple(s *State, node *ast.Node, exp luau.Expression) luau.Expression {
	if IsLuaTupleType(s).Check(s.GetType(node)) && shouldWrapLuaTuple(s, node, exp) {
		return luau.NewArray(luau.NewList[luau.Expression](exp))
	}
	return exp
}

// ---------------------------------------------------------------------------
// fixVoidArgumentsForRobloxFunctions — transformCallExpression.ts L96-113
// ---------------------------------------------------------------------------

// fixVoidArgumentsForRobloxFunctions wraps possibly-undefined call-expression
// arguments of Roblox API functions in parentheses: `(foo())` truncates Lua
// multi-returns/void so C functions like `tonumber()` don't error on zero
// values.
func fixVoidArgumentsForRobloxFunctions(s *State, expType *checker.Type, args []luau.Expression, nodeArguments []*ast.Node) {
	if !IsPossiblyType(s, expType, IsRobloxType(s)) {
		return
	}
	for i, nodeArg := range nodeArguments {
		if ast.IsCallExpression(nodeArg) && IsPossiblyType(s, s.GetType(nodeArg), IsUndefinedType) {
			args[i] = luau.NewParenthesized(args[i])
		}
	}
}

// ---------------------------------------------------------------------------
// The three call transforms — transformCallExpression.ts
// ---------------------------------------------------------------------------

// transformCallExpressionInner ports transformCallExpressionInner (L115-155)
// — plain `f(...)`.
func transformCallExpressionInner(s *State, node *ast.Node, expression luau.Expression, nodeArguments []*ast.Node) luau.Expression {
	if ast.IsImportCall(node) {
		// transformImportExpression: dynamic import lands with the module task.
		s.Diags.Add(DiagRotorNotYetSupported(node, "dynamic `import()`"))
		return luau.NewNone()
	}

	call := node.AsCallExpression()

	// a in a()
	validateNotAnyType(s, call.Expression)

	if ast.IsSuperCall(node) {
		// super.constructor(self, ...) needs the class transforms: Phase 3.
		s.Diags.Add(DiagRotorNotYetSupported(node, "`super` calls"))
		return luau.NewNone()
	}

	expType := s.Checker.GetNonOptionalType(s.GetType(call.Expression))
	if symbol := GetFirstDefinedSymbol(s, expType); symbol != nil {
		if macro := s.Macros().GetCallMacro(symbol); macro != nil {
			// upstream: runCallMacro(macro, state, node, expression,
			// nodeArguments) — lands with the Phase 3b macro tables; every
			// registered entry is a sentinel until then.
			s.Diags.Add(DiagRotorNotYetSupported(node, "macro `"+macro.Name+"`"))
			return luau.NewNone()
		}
	}

	var args []luau.Expression
	prereqs := s.CaptureStatements(func() { args = ensureTransformOrder(s, nodeArguments) })
	fixVoidArgumentsForRobloxFunctions(s, expType, args, nodeArguments)

	if prereqs.IsNonEmpty() && expressionMightMutate(s, expression, call.Expression) {
		expression = s.PushToVar(expression, "fn")
	}
	s.PrereqList(prereqs)

	exp := luau.NewCall(convertToIndexableExpression(expression), luau.NewList(args...))

	return wrapReturnIfLuaTuple(s, node, exp)
}

// transformPropertyCallExpressionInner ports
// transformPropertyCallExpressionInner (L157-215) — `a.b(...)`. expression is
// the ts PropertyAccessExpression; baseExpression the already-transformed `a`.
func transformPropertyCallExpressionInner(s *State, node *ast.Node, expression *ast.Node, baseExpression luau.Expression, name string, nodeArguments []*ast.Node) luau.Expression {
	propertyAccess := expression.AsPropertyAccessExpression()
	call := node.AsCallExpression()

	// a in a.b()
	validateNotAnyType(s, propertyAccess.Expression)
	// a.b in a.b()
	validateNotAnyType(s, call.Expression)

	if ast.IsSuperProperty(expression) {
		s.Diags.Add(DiagRotorNotYetSupported(node, "`super` calls"))
		return luau.NewNone()
	}

	expType := s.Checker.GetNonOptionalType(s.GetType(call.Expression))
	if symbol := GetFirstDefinedSymbol(s, expType); symbol != nil {
		if macro := s.Macros().GetPropertyCallMacro(symbol); macro != nil {
			// upstream: runCallMacro(macro, state, node, baseExpression,
			// nodeArguments) — Phase 3b; sentinel until then.
			s.Diags.Add(DiagRotorNotYetSupported(node, "macro `"+macro.Name+"`"))
			return luau.NewNone()
		}
	}

	var args []luau.Expression
	prereqs := s.CaptureStatements(func() { args = ensureTransformOrder(s, nodeArguments) })
	fixVoidArgumentsForRobloxFunctions(s, expType, args, nodeArguments)

	if prereqs.IsNonEmpty() && expressionMightMutate(s, baseExpression, propertyAccess.Expression) {
		baseExpression = s.PushToVar(baseExpression, "")
	}
	s.PrereqList(prereqs)

	var exp luau.Expression
	if isMethod(s, expression) {
		// check that the name isn't a Luau keyword
		// if it is, we need to use PropertyAccessExpression and manually add the self argument
		if luau.IsValidIdentifier(name) {
			exp = luau.NewMethodCall(name, convertToIndexableExpression(baseExpression), luau.NewList(args...))
		} else {
			baseExpression = s.PushToVarIfComplex(baseExpression, "")
			args = append([]luau.Expression{baseExpression}, args...)
			exp = luau.NewCall(
				luau.NewPropertyAccess(convertToIndexableExpression(baseExpression), name),
				luau.NewList(args...),
			)
		}
	} else {
		// PropertyAccessExpression will wrap the identifier for us if necessary
		exp = luau.NewCall(
			luau.NewPropertyAccess(convertToIndexableExpression(baseExpression), name),
			luau.NewList(args...),
		)
	}

	return wrapReturnIfLuaTuple(s, node, exp)
}

// transformElementCallExpressionInner ports
// transformElementCallExpressionInner (L217-280) — `a[b](...)`. NOTE the
// index expression is ordered WITH the arguments, and `a[b]` can never use
// `:` sugar: methods always get an explicit self argument.
func transformElementCallExpressionInner(s *State, node *ast.Node, expression *ast.Node, baseExpression luau.Expression, argumentExpression *ast.Node, nodeArguments []*ast.Node) luau.Expression {
	elementAccess := expression.AsElementAccessExpression()
	call := node.AsCallExpression()

	// a in a[b]()
	validateNotAnyType(s, elementAccess.Expression)
	// b in a[b]()
	validateNotAnyType(s, elementAccess.ArgumentExpression)
	// a[b] in a[b]()
	validateNotAnyType(s, call.Expression)

	if ast.IsSuperProperty(expression) {
		s.Diags.Add(DiagRotorNotYetSupported(node, "`super` calls"))
		return luau.NewNone()
	}

	expType := s.Checker.GetNonOptionalType(s.GetType(call.Expression))
	if symbol := GetFirstDefinedSymbol(s, expType); symbol != nil {
		if macro := s.Macros().GetPropertyCallMacro(symbol); macro != nil {
			// upstream: runCallMacro(macro, state, node, baseExpression,
			// nodeArguments) — Phase 3b; sentinel until then.
			s.Diags.Add(DiagRotorNotYetSupported(node, "macro `"+macro.Name+"`"))
			return luau.NewNone()
		}
	}

	var ordered []luau.Expression
	prereqs := s.CaptureStatements(func() {
		ordered = ensureTransformOrder(s, append([]*ast.Node{argumentExpression}, nodeArguments...))
	})
	argumentExp, args := ordered[0], ordered[1:]

	fixVoidArgumentsForRobloxFunctions(s, expType, args, nodeArguments)

	if prereqs.IsNonEmpty() && expressionMightMutate(s, baseExpression, elementAccess.Expression) {
		baseExpression = s.PushToVar(baseExpression, "")
	}
	s.PrereqList(prereqs)

	if isMethod(s, expression) {
		baseExpression = s.PushToVarIfComplex(baseExpression, "")
		args = append([]luau.Expression{baseExpression}, args...)
	}

	exp := luau.NewCall(
		luau.NewComputedIndex(
			convertToIndexableExpression(baseExpression),
			addOneIfArrayType(s,
				s.Checker.GetNonOptionalType(s.GetType(elementAccess.Expression)),
				argumentExp),
		),
		luau.NewList(args...),
	)

	return wrapReturnIfLuaTuple(s, node, exp)
}

// transformCallExpression ports transformCallExpression (L282-284): every
// call routes through the optional-chain walker.
func transformCallExpression(s *State, node *ast.Node) luau.Expression {
	return transformOptionalChain(s, node)
}

// ---------------------------------------------------------------------------
// Optional chain — nodes/transformOptionalChain.ts (non-optional fold)
// ---------------------------------------------------------------------------

type chainItemKind int

const (
	chainPropertyAccess chainItemKind = iota
	chainElementAccess
	chainCall
	chainPropertyCall
	chainElementCall
)

// chainItem ports the OptionalChainItem family (L24-66). The eager type
// snapshots upstream records are consumed only by the optional path and are
// omitted; the non-optional inners re-query the (memoized) checker.
type chainItem struct {
	kind     chainItemKind
	node     *ast.Node // the PropertyAccess/ElementAccess/Call expression
	optional bool

	name               string    // PropertyAccess / PropertyCall
	argumentExpression *ast.Node // ElementAccess / ElementCall

	// compound call items (PropertyCall / ElementCall)
	expression   *ast.Node // the inner access expression
	callOptional bool
	args         []*ast.Node
}

// flattenOptionalChain ports flattenOptionalChain (L135-162): walk down
// `.expression` collecting chain items; a CallExpression whose callee (skipped
// downwards) is a property/element access becomes a compound item.
func flattenOptionalChain(expression *ast.Node) ([]chainItem, *ast.Node) {
	var chain []chainItem // built outer-to-inner, reversed below (upstream unshifts)
	for {
		if ast.IsPropertyAccessExpression(expression) {
			propertyAccess := expression.AsPropertyAccessExpression()
			chain = append(chain, chainItem{
				kind:     chainPropertyAccess,
				node:     expression,
				optional: propertyAccess.QuestionDotToken != nil,
				name:     propertyAccess.Name().Text(),
			})
			expression = propertyAccess.Expression
		} else if ast.IsElementAccessExpression(expression) {
			elementAccess := expression.AsElementAccessExpression()
			chain = append(chain, chainItem{
				kind:               chainElementAccess,
				node:               expression,
				optional:           elementAccess.QuestionDotToken != nil,
				argumentExpression: elementAccess.ArgumentExpression,
			})
			expression = elementAccess.Expression
		} else if ast.IsCallExpression(expression) {
			// this is a bit of a mess..
			call := expression.AsCallExpression()
			subExp := SkipDownwards(call.Expression)
			if ast.IsPropertyAccessExpression(subExp) {
				propertyAccess := subExp.AsPropertyAccessExpression()
				chain = append(chain, chainItem{
					kind:         chainPropertyCall,
					node:         expression,
					expression:   subExp,
					optional:     propertyAccess.QuestionDotToken != nil,
					name:         propertyAccess.Name().Text(),
					callOptional: call.QuestionDotToken != nil,
					args:         call.Arguments.Nodes,
				})
				expression = propertyAccess.Expression
			} else if ast.IsElementAccessExpression(subExp) {
				elementAccess := subExp.AsElementAccessExpression()
				chain = append(chain, chainItem{
					kind:               chainElementCall,
					node:               expression,
					expression:         subExp,
					optional:           elementAccess.QuestionDotToken != nil,
					argumentExpression: elementAccess.ArgumentExpression,
					callOptional:       call.QuestionDotToken != nil,
					args:               call.Arguments.Nodes,
				})
				expression = elementAccess.Expression
			} else {
				chain = append(chain, chainItem{
					kind:     chainCall,
					node:     expression,
					optional: call.QuestionDotToken != nil,
					args:     call.Arguments.Nodes,
				})
				expression = subExp
			}
		} else {
			break
		}
	}
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	return chain, expression
}

// transformChainItem ports transformChainItem (L164-190): dispatch to the
// matching Inner transform.
func transformChainItem(s *State, baseExpression luau.Expression, item chainItem) luau.Expression {
	switch item.kind {
	case chainPropertyAccess:
		return transformPropertyAccessExpressionInner(s, item.node, baseExpression, item.name)
	case chainElementAccess:
		return transformElementAccessExpressionInner(s, item.node, baseExpression, item.argumentExpression)
	case chainCall:
		return transformCallExpressionInner(s, item.node, baseExpression, item.args)
	case chainPropertyCall:
		return transformPropertyCallExpressionInner(s, item.node, item.expression, baseExpression, item.name, item.args)
	default: // chainElementCall
		return transformElementCallExpressionInner(s, item.node, item.expression, baseExpression, item.argumentExpression, item.args)
	}
}

// transformOptionalChain ports transformOptionalChain (L350-356) restricted
// to fully non-optional chains, where transformOptionalChainInner (L339-347)
// degenerates to an inner-to-outer left fold of transformChainItem. A real
// `?.` (the optional path's nested nil-check blocks, L191-348) raises
// rotorNotYetSupported.
func transformOptionalChain(s *State, node *ast.Node) luau.Expression {
	chain, expression := flattenOptionalChain(node)
	for _, item := range chain {
		if item.optional || item.callOptional {
			s.Diags.Add(DiagRotorNotYetSupported(item.node, "optional chaining (`?.`)"))
			return luau.NewNone()
		}
	}
	result := TransformExpression(s, expression)
	for _, item := range chain {
		result = transformChainItem(s, result, item)
	}
	return result
}
