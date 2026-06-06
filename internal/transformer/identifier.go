package transformer

import (
	"rotor/internal/luau"
	"rotor/tsgo/ast"
)

// TransformIdentifierDefined ports transformIdentifier.ts
// transformIdentifierDefined: an identifier in a *defining* position. The
// symbol is looked up (shorthand-property-assignment aware) so renamed/
// captured variables resolve through SymbolToID; otherwise the identifier's
// own text is used.
//
// Task 7 adds the full transformIdentifier (use positions: undefined->nil,
// macro diagnostics, hoist checking, export-let routing) alongside this.
func TransformIdentifierDefined(s *State, node *ast.Node) luau.AnyIdentifier {
	var symbol *ast.Symbol
	if node.Parent != nil && ast.IsShorthandPropertyAssignment(node.Parent) {
		symbol = s.Checker.GetShorthandAssignmentValueSymbol(node.Parent)
	} else {
		symbol = s.Checker.GetSymbolAtLocation(node)
	}
	if symbol == nil {
		panic("transformer: TransformIdentifierDefined: no symbol") // upstream assert
	}
	if replacement := s.SymbolToID[symbol]; replacement != nil {
		return replacement
	}
	return luau.ID(node.Text())
}
