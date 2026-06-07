package transformer

import (
	"rotor/internal/luau"
	"rotor/tsgo/ast"
)

// This file ports nodes/class/transformDecorators.ts — LEGACY experimental
// decorators only (rbxtsc 3.0 requires "experimentalDecorators": true; under
// it tsgo's checker runs in legacyDecorators mode and decorators parse into
// the Modifiers list).
//
// Shapes (oracle-pinned, 2026-06-07):
//
//	-- method
//	local _descriptor = decorator(Class, KEY, {
//		value = Class[KEY],
//	})
//	if _descriptor then
//		Class[KEY] = _descriptor.value
//	end
//	-- property
//	decorator(Class, KEY)
//	-- parameter (0-based index; KEY is nil for constructor parameters)
//	decorator(Class, KEY, i)
//	-- class
//	Class = decorator(Class) or Class
//
// Evaluation order: decorator EXPRESSIONS initialize first-to-last (source
// order; a mutable expression spills to `local _decorator = ...` unless
// shouldInlineDecorator allows inlining), applications run LAST-TO-FIRST
// (finalizers unshift — TC39 bottom-up order). Parameter decorators sandwich
// between their member's initializers and finalizers.

// getDecorators collects the Decorator nodes off the Modifiers list
// (ts.getDecorators; tsgo keeps decorators among the modifiers).
func getDecorators(node *ast.Node) []*ast.Node {
	modifiers := node.Modifiers()
	if modifiers == nil {
		return nil
	}
	var decorators []*ast.Node
	for _, modifier := range modifiers.Nodes {
		if modifier.Kind == ast.KindDecorator {
			decorators = append(decorators, modifier)
		}
	}
	return decorators
}

// countDecorators ports transformDecorators.ts countDecorators (L13-15).
func countDecorators(node *ast.Node) int {
	return len(getDecorators(node))
}

// anyParameterHasDecorators reports whether any parameter of a function-like
// node carries a decorator.
func anyParameterHasDecorators(node *ast.Node) bool {
	for _, parameter := range node.Parameters() {
		if countDecorators(parameter) > 0 {
			return true
		}
	}
	return false
}

// shouldInlineDecorator ports transformDecorators.ts shouldInline (L17-56).
func shouldInlineDecorator(s *State, isLastDecorator bool, decorator *ast.Node, expression luau.Expression) bool {
	// immutable expressions can be inlined
	if !expressionMightMutate(s, expression, decorator.Expression()) {
		return true
	}

	// if it's not the last decorator, we can't inline
	// this is because we need to initialize all decorators before running them
	if !isLastDecorator {
		return false
	}

	node := decorator.Parent

	// if the node is a method declaration and has a decorator on a parameter,
	// we can't inline: parameter decorators run between initializing and
	// running method decorators
	if ast.IsMethodDeclaration(node) && anyParameterHasDecorators(node) {
		return false
	}

	// if the node is a class declaration and has a decorator on a constructor
	// parameter, we can't inline: constructor parameter decorators run between
	// initializing and running class decorators
	if ast.IsClassLike(node) {
		if constructor := findConstructor(node); constructor != nil && anyParameterHasDecorators(constructor) {
			return false
		}
	}

	// if the node is a parameter and there are any parameters with decorators
	// after it, we can't inline: all of the parameters must be initialized
	// before running any, including from sibling parameters
	if ast.IsParameterDeclaration(node) {
		parameters := node.Parent.Parameters()
		paramIdx := -1
		for i, parameter := range parameters {
			if parameter == node {
				paramIdx = i
				break
			}
		}
		for i := paramIdx + 1; i < len(parameters); i++ {
			if countDecorators(parameters[i]) > 0 {
				return false
			}
		}
	}

	return true
}

// transformMemberDecorators ports transformDecorators.ts
// transformMemberDecorators (L58-92) — the shared driver. Per decorator in
// SOURCE order: capture the transformed expression (prereqs join the
// initializers), spill to a `local _decorator = ...` temp unless inlinable,
// and UNSHIFT the callback's statements into the finalizers so application
// runs last-to-first.
func transformMemberDecorators(
	s *State,
	node *ast.Node,
	callback func(expression luau.IndexableExpression) *luau.List[luau.Statement],
) (initializers, finalizers *luau.List[luau.Statement]) {
	initializers = luau.NewList[luau.Statement]()
	finalizers = luau.NewList[luau.Statement]()

	decorators := getDecorators(node)

	for i, decorator := range decorators {
		expression, prereqs := s.Capture(func() luau.Expression {
			return TransformExpression(s, decorator.Expression())
		})

		initializers.PushList(prereqs)

		isLastDecorator := i == len(decorators)-1

		if !shouldInlineDecorator(s, isLastDecorator, decorator, expression) {
			tempID := luau.TempID("decorator")
			initializers.Push(luau.NewVariableDeclaration(tempID, expression))
			expression = tempID
		}

		finalizers.UnshiftList(callback(convertToIndexableExpression(expression)))
	}

	return initializers, finalizers
}

// transformMethodDecorators ports transformDecorators.ts
// transformMethodDecorators (L94-157). The member's parameter decorators run
// between the method decorators' initializers and finalizers.
func transformMethodDecorators(s *State, member *ast.Node, classID luau.AnyIdentifier) *luau.List[luau.Statement] {
	initializers, finalizers := transformMemberDecorators(s, member, func(expression luau.IndexableExpression) *luau.List[luau.Statement] {
		result := luau.NewList[luau.Statement]()

		// local _descriptor = decorator(Class, "name", { value = Class.name })
		// if _descriptor then
		// 	Class.name = _descriptor.value
		// end

		descriptorID := luau.TempID("descriptor")
		key := s.GetClassElementObjectKey(member)
		if key == nil {
			panic("transformer: did not find method key for method decorator") // upstream assert
		}

		result.Push(luau.NewVariableDeclaration(descriptorID, luau.NewCall(expression, luau.NewList[luau.Expression](
			classID,
			key,
			luau.NewMap(luau.NewList(
				luau.NewMapField(luau.Str("value"), luau.NewComputedIndex(classID, key)),
			)),
		))))

		result.Push(luau.NewIf(
			descriptorID,
			luau.NewList[luau.Statement](luau.NewAssignment(
				luau.NewComputedIndex(classID, key),
				"=",
				luau.NewPropertyAccess(descriptorID, "value"),
			)),
			nil,
		))

		return result
	})

	result := luau.NewList[luau.Statement]()
	result.PushList(initializers)
	result.PushList(transformParameterDecorators(s, member, classID))
	result.PushList(finalizers)
	return result
}

// transformPropertyDecorators ports transformDecorators.ts
// transformPropertyDecorators (L159-180).
func transformPropertyDecorators(s *State, member *ast.Node, classID luau.AnyIdentifier) *luau.List[luau.Statement] {
	initializers, finalizers := transformMemberDecorators(s, member, func(expression luau.IndexableExpression) *luau.List[luau.Statement] {
		// typescript enforces that property keys are static, so they
		// shouldn't have prereqs
		key := s.NoPrereqs(func() luau.Expression {
			return transformPropertyName(s, member.Name())
		})

		// decorator(Class, "name")
		return luau.NewList[luau.Statement](luau.NewCallStatement(
			luau.NewCall(expression, luau.NewList[luau.Expression](classID, key)),
		))
	})

	result := luau.NewList[luau.Statement]()
	result.PushList(initializers)
	result.PushList(finalizers)
	return result
}

// transformParameterDecorators ports transformDecorators.ts
// transformParameterDecorators (L182-212). member is a MethodDeclaration or
// ConstructorDeclaration. Parameter finalizers ALSO unshift, so the last
// parameter's decorators apply first.
func transformParameterDecorators(s *State, member *ast.Node, classID luau.AnyIdentifier) *luau.List[luau.Statement] {
	initializers := luau.NewList[luau.Statement]()
	finalizers := luau.NewList[luau.Statement]()

	for i, parameter := range member.Parameters() {
		paramInitializers, paramFinalizers := transformMemberDecorators(s, parameter, func(expression luau.IndexableExpression) *luau.List[luau.Statement] {
			// No member.name means it's the constructor, so the name argument
			// should be nil
			var key luau.Expression
			if member.Name() != nil {
				key = s.GetClassElementObjectKey(member)
				if key == nil {
					panic("transformer: did not find method key for parameter decorator") // upstream assert
				}
			} else {
				key = luau.Nil()
			}

			// decorator(Class, "name", 0)
			return luau.NewList[luau.Statement](luau.NewCallStatement(
				luau.NewCall(expression, luau.NewList[luau.Expression](classID, key, luau.Num(float64(i)))),
			))
		})
		initializers.PushList(paramInitializers)
		finalizers.UnshiftList(paramFinalizers)
	}

	result := luau.NewList[luau.Statement]()
	result.PushList(initializers)
	result.PushList(finalizers)
	return result
}

// transformClassDecorators ports transformDecorators.ts
// transformClassDecorators (L214-240). Constructor parameter decorators run
// between the class decorators' initializers and finalizers.
func transformClassDecorators(s *State, node *ast.Node, classID luau.AnyIdentifier) *luau.List[luau.Statement] {
	initializers, finalizers := transformMemberDecorators(s, node, func(expression luau.IndexableExpression) *luau.List[luau.Statement] {
		// Class = decorator(Class) or Class
		return luau.NewList[luau.Statement](luau.NewAssignment(
			classID,
			"=",
			luau.NewBinary(luau.NewCall(expression, luau.NewList[luau.Expression](classID)), "or", classID),
		))
	})

	result := luau.NewList[luau.Statement]()
	result.PushList(initializers)

	if constructor := findConstructor(node); constructor != nil {
		result.PushList(transformParameterDecorators(s, constructor, classID))
	}

	result.PushList(finalizers)
	return result
}

// transformDecorators ports transformDecorators.ts transformDecorators
// (L242-279), called as the LAST thing inside the class do-block (with the
// RETURN var, not the internal name). Order per the TS handbook's decorator
// evaluation rules: instance members (declaration order), then static members
// (declaration order), then class decorators (with constructor parameter
// decorators sandwiched). Methods without bodies are overload signatures —
// decorators are not valid on them. Accessors never reach here (noGetterSetter
// fires first; upstream excludes ts.AccessorDeclaration from HasDecorators).
func transformDecorators(s *State, node *ast.Node, classID luau.AnyIdentifier) *luau.List[luau.Statement] {
	result := luau.NewList[luau.Statement]()

	// Instance Decorators
	for _, member := range node.Members() {
		if !ast.HasStaticModifier(member) {
			if ast.IsMethodDeclaration(member) && member.Body() != nil {
				result.PushList(transformMethodDecorators(s, member, classID))
			} else if ast.IsPropertyDeclaration(member) {
				result.PushList(transformPropertyDecorators(s, member, classID))
			}
		}
	}

	// Static Decorators
	for _, member := range node.Members() {
		if ast.HasStaticModifier(member) {
			if ast.IsMethodDeclaration(member) && member.Body() != nil {
				result.PushList(transformMethodDecorators(s, member, classID))
			} else if ast.IsPropertyDeclaration(member) {
				result.PushList(transformPropertyDecorators(s, member, classID))
			}
		}
	}

	// Class Decorators
	result.PushList(transformClassDecorators(s, node, classID))

	return result
}
