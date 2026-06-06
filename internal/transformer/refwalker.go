package transformer

import (
	"rotor/tsgo/ast"
	"rotor/tsgo/checker"
)

// This file ports the ts.FindAllReferences.Core internals the transformer
// consumes (eachSymbolReferenceInFile / isSymbolReferencedInFile), scoped to
// the single-file callers upstream has:
//   - util/checkVariableHoist.ts:23 (searchContainer = the CaseBlock)
//   - statements/transformForStatement.ts:79 (the ForStatement), :75 (the
//     VariableDeclarationList), :350 (the loop body)
//
// All call sites pass a `definition` identifier that IS a declaration name
// and search a container within the same source file, so no cross-file or
// alias-following semantics are needed.

// ForEachSymbolReference walks searchContainer's subtree in document order,
// invoking onRef for every Identifier that references definition's symbol.
// onRef returning true terminates the walk early; the return value reports
// whether any onRef call returned true (upstream eachSymbolReferenceInFile's
// truthy result).
//
// Per the upstream semantics: the definition identifier itself is skipped; a
// cheap text prefilter (token text == definition text) runs before symbol
// resolution; matches are symbol identity via getSymbolAtLocation OR the
// shorthand-property value symbol (`{ x }` references x). The third upstream
// match arm — export specifiers resolved through their local symbol — is
// deliberately omitted: every rotor call site searches block-scoped locals
// inside a ForStatement/CaseBlock container, where export specifiers cannot
// occur (documented divergence).
func ForEachSymbolReference(chk *checker.Checker, definition *ast.Node, searchContainer *ast.Node, onRef func(token *ast.Node) bool) bool {
	symbol := chk.GetSymbolAtLocation(definition)
	if symbol == nil {
		return false
	}
	name := definition.Text()

	var visit func(node *ast.Node) bool
	visit = func(node *ast.Node) bool {
		if ast.IsIdentifier(node) {
			if node == definition || node.Text() != name {
				return false
			}
			matches := chk.GetSymbolAtLocation(node) == symbol
			if !matches && node.Parent != nil && ast.IsShorthandPropertyAssignment(node.Parent) {
				matches = chk.GetShorthandAssignmentValueSymbol(node.Parent) == symbol
			}
			if matches {
				return onRef(node)
			}
			return false
		}
		return node.ForEachChild(visit)
	}
	return visit(searchContainer)
}

// IsSymbolReferenced ports isSymbolReferencedInFile: true when definition's
// symbol has at least one reference (besides the definition itself) inside
// searchContainer.
func IsSymbolReferenced(chk *checker.Checker, definition *ast.Node, searchContainer *ast.Node) bool {
	return ForEachSymbolReference(chk, definition, searchContainer, func(*ast.Node) bool { return true })
}
