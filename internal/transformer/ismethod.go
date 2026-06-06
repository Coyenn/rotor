package transformer

import (
	"rotor/tsgo/ast"
	"rotor/tsgo/checker"
)

// This file ports TSTransformer/util/isMethod.ts (complete).

// getThisParameter ports isMethod.ts getThisParameter (L9-17): the first
// parameter's name when it is the explicit `this` identifier.
func getThisParameter(parameters []*ast.Node) *ast.Node {
	if len(parameters) > 0 {
		name := parameters[0].Name()
		if name != nil && ast.IsIdentifier(name) && ast.IsThisIdentifier(name) {
			return name
		}
	}
	return nil
}

// isMethodDeclarationNode ports isMethod.ts isMethodDeclaration (L19-49): is
// this signature declaration a method (callable with `:`)?
func isMethodDeclarationNode(s *State, node *ast.Node) bool {
	if !ast.IsFunctionLike(node) {
		return false
	}

	if thisParam := getThisParameter(node.Parameters()); thisParam != nil {
		return s.GetType(thisParam).Flags()&checker.TypeFlagsVoid == 0
	}

	// namespace declare functions with `this` arg defined (i.e. utf8)
	if ast.IsFunctionDeclaration(node) {
		return false
	}

	if ast.IsMethodDeclaration(node) || ast.IsMethodSignatureDeclaration(node) {
		return true
	}

	// for some reason, FunctionExpressions within ObjectLiteralExpressions are implicitly methods
	if ast.IsFunctionExpression(node) {
		parent := SkipUpwards(node).Parent
		if parent != nil && ast.IsPropertyAssignment(parent) {
			grandparent := SkipUpwards(parent).Parent
			if grandparent != nil && ast.IsObjectLiteralExpression(grandparent) {
				return true
			}
		}
	}

	return false
}

// isMethodInner ports isMethod.ts isMethodInner (L51-77): scan the call
// signatures — an explicit `this` parameter typed void marks a callback, any
// other `this` type marks a method; signatures without a thisParameter symbol
// fall back to their declaration's shape. Mixing both on one type is a
// noMixedTypeCall error.
func isMethodInner(s *State, node *ast.Node, t *checker.Type) bool {
	hasMethodDefinition := false
	hasCallbackDefinition := false

	for _, callSignature := range s.Checker.GetCallSignatures(t) {
		var thisValueDeclaration *ast.Node
		if thisParameter := callSignature.ThisParameter(); thisParameter != nil {
			thisValueDeclaration = thisParameter.ValueDeclaration
		}
		if thisValueDeclaration != nil {
			if s.GetType(thisValueDeclaration).Flags()&checker.TypeFlagsVoid == 0 {
				hasMethodDefinition = true
			} else {
				hasCallbackDefinition = true
			}
		} else if declaration := callSignature.Declaration(); declaration != nil {
			if isMethodDeclarationNode(s, declaration) {
				hasMethodDefinition = true
			} else {
				hasCallbackDefinition = true
			}
		}
	}

	if hasMethodDefinition && hasCallbackDefinition {
		s.Diags.Add(DiagNoMixedTypeCall(node))
	}

	return hasMethodDefinition
}

// isMethodFromType ports isMethod.ts isMethodFromType (L79-91): walk the
// type's union/intersection members and constraints; each leaf with a symbol
// consults the program-wide per-symbol cache (multiTransformState.isMethodCache).
func isMethodFromType(s *State, node *ast.Node, t *checker.Type) bool {
	result := false
	WalkTypes(s, t, func(t *checker.Type) {
		// NOTE upstream `result ||= getOrSetDefault(...)` short-circuits: once
		// result is true, later leaves are neither checked nor cached (and
		// cannot add noMixedTypeCall diagnostics) — preserved exactly.
		if result {
			return
		}
		if symbol := t.Symbol(); symbol != nil {
			cached, ok := s.Multi.IsMethodCache[symbol]
			if !ok {
				cached = isMethodInner(s, node, t)
				s.Multi.IsMethodCache[symbol] = cached
			}
			result = result || cached
		}
	})
	return result
}

// isMethod ports isMethod.ts isMethod (L93-98).
func isMethod(s *State, node *ast.Node) bool {
	return isMethodFromType(s, node, s.GetType(node))
}
