package transformer

import (
	"rotor/internal/luau"
	"rotor/tsgo/ast"
)

// This file ports the object-pattern destructuring transforms:
// nodes/binding/transformObjectBindingPattern.ts and
// nodes/binding/transformObjectAssignmentPattern.ts.

// ---------------------------------------------------------------------------
// Binding patterns — `const { a, b: c } = exp`
// ---------------------------------------------------------------------------

// transformObjectBindingPattern ports transformObjectBindingPattern.ts
// (L13-48). A rest element (`...rest`) raises noSpreadDestructuring and
// ABORTS the remaining elements. `{ a: b }` reads property `a` and declares
// `b`; a nested pattern name guarantees a propertyName (upstream
// `assert(prop)`).
func transformObjectBindingPattern(s *State, bindingPattern *ast.Node, parentID luau.AnyIdentifier) {
	validateNotAnyType(s, bindingPattern)
	for _, element := range bindingPattern.AsBindingPattern().Elements.Nodes {
		bindingElement := element.AsBindingElement()
		if bindingElement.DotDotDotToken != nil {
			s.Diags.Add(DiagNoSpreadDestructuring(element))
			return
		}
		name := bindingElement.Name()
		prop := bindingElement.PropertyName
		if ast.IsIdentifier(name) {
			nameOrProp := prop
			if nameOrProp == nil {
				nameOrProp = name
			}
			value := objectAccessor(s, parentID, s.GetType(bindingPattern), nameOrProp)
			id := transformVariable(s, name, value)
			if bindingElement.Initializer != nil {
				s.Prereq(transformInitializer(s, id, bindingElement.Initializer))
			}
		} else {
			// if name is not identifier, it must be a binding pattern
			// in that case, prop is guaranteed to exist
			if prop == nil {
				panic("transformer: transformObjectBindingPattern: nested pattern without property name") // upstream assert
			}
			value := objectAccessor(s, parentID, s.GetType(bindingPattern), prop)
			id := s.PushToVar(value, "binding")
			if bindingElement.Initializer != nil {
				s.Prereq(transformInitializer(s, id, bindingElement.Initializer))
			}
			if ast.IsArrayBindingPattern(name) {
				transformArrayBindingPattern(s, name, id)
			} else {
				transformObjectBindingPattern(s, name, id)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Assignment patterns — `({ a, b: target } = exp)`
// ---------------------------------------------------------------------------

// transformObjectAssignmentPattern ports transformObjectAssignmentPattern.ts
// (L14-90), over an ObjectLiteralExpression LHS. The pattern type comes from
// the checker's getTypeOfAssignmentPattern (NOT getType). Shorthand defaults
// arrive via objectAssignmentInitializer (`{ a = 1 }`); property-assignment
// defaults as BinaryExpression initializers (`{ a: b = 1 }`). A
// SpreadAssignment raises noSpreadDestructuring and ABORTS the remaining
// properties.
func transformObjectAssignmentPattern(s *State, assignmentPattern *ast.Node, parentID luau.AnyIdentifier) {
	for _, property := range assignmentPattern.AsObjectLiteralExpression().Properties.Nodes {
		if ast.IsShorthandPropertyAssignment(property) {
			shorthand := property.AsShorthandPropertyAssignment()
			name := shorthand.Name()
			value := objectAccessor(s, parentID,
				s.Checker.GetTypeOfAssignmentPattern(assignmentPattern), name)
			id := transformWritableExpression(s, name, shorthand.ObjectAssignmentInitializer != nil)
			s.Prereq(luau.NewAssignment(id, "=", value))
			if _, isAnyIdentifier := id.(luau.AnyIdentifier); !isAnyIdentifier {
				panic("transformer: transformObjectAssignmentPattern: shorthand target is not an identifier") // upstream assert
			}
			if shorthand.ObjectAssignmentInitializer != nil {
				s.Prereq(transformInitializer(s, id, shorthand.ObjectAssignmentInitializer))
			}
		} else if ast.IsSpreadAssignment(property) {
			s.Diags.Add(DiagNoSpreadDestructuring(property))
			return
		} else if ast.IsPropertyAssignment(property) {
			propertyAssignment := property.AsPropertyAssignment()
			name := propertyAssignment.Name()
			init := propertyAssignment.Initializer
			var initializer *ast.Node
			if ast.IsBinaryExpression(propertyAssignment.Initializer) {
				binary := propertyAssignment.Initializer.AsBinaryExpression()
				initializer = SkipDownwards(binary.Right)
				init = SkipDownwards(binary.Left)
			}

			value := objectAccessor(s, parentID,
				s.Checker.GetTypeOfAssignmentPattern(assignmentPattern), name)
			if ast.IsIdentifier(init) || ast.IsElementAccessExpression(init) || ast.IsPropertyAccessExpression(init) {
				id := transformWritableExpression(s, init, initializer != nil)
				s.Prereq(luau.NewAssignment(id, "=", value))
				if initializer != nil {
					s.Prereq(transformInitializer(s, id, initializer))
				}
			} else if ast.IsArrayLiteralExpression(init) {
				id := s.PushToVar(value, "binding")
				if initializer != nil {
					s.Prereq(transformInitializer(s, id, initializer))
				}
				if !ast.IsIdentifier(name) {
					panic("transformer: transformObjectAssignmentPattern: nested array pattern with non-identifier name") // upstream assert
				}
				transformArrayAssignmentPattern(s, init, id)
			} else if ast.IsObjectLiteralExpression(init) {
				id := s.PushToVar(value, "binding")
				if initializer != nil {
					s.Prereq(transformInitializer(s, id, initializer))
				}
				transformObjectAssignmentPattern(s, init, id)
			} else {
				panic("transformer: transformObjectAssignmentPattern invalid initializer: " + kindName(init.Kind)) // upstream assert
			}
		} else {
			panic("transformer: transformObjectAssignmentPattern invalid property: " + kindName(property.Kind)) // upstream assert
		}
	}
}
