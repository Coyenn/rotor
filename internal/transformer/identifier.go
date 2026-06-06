package transformer

import (
	"rotor/internal/luau"
	"rotor/tsgo/ast"
)

// TransformIdentifier ports transformIdentifier.ts transformIdentifier (use
// positions), minimal path: shorthand-aware symbol lookup, undefined -> nil
// mapping, then the defined form. Task 7 adds the rest of the reference path:
// noArguments/noGlobalThis special symbols, identifier/constructor/call macro
// misuse diagnostics, `export let` indirection, and checkIdentifierHoist.
func TransformIdentifier(s *State, node *ast.Node) luau.Expression {
	var symbol *ast.Symbol
	if node.Parent != nil && ast.IsShorthandPropertyAssignment(node.Parent) {
		symbol = s.Checker.GetShorthandAssignmentValueSymbol(node.Parent)
	} else {
		symbol = s.Checker.GetSymbolAtLocation(node)
	}
	if symbol == nil {
		panic("transformer: TransformIdentifier: no symbol") // upstream assert
	}
	if s.Checker.IsUndefinedSymbol(symbol) {
		return luau.Nil()
	}
	return TransformIdentifierDefined(s, node)
}

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
