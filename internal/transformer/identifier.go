package transformer

import (
	"strings"

	"rotor/internal/luau"
	"rotor/tsgo/ast"
)

// identifierSymbol is the shorthand-aware symbol lookup shared by
// TransformIdentifier and TransformIdentifierDefined (transformIdentifier.ts
// L15-18/L119-122): an identifier inside a ShorthandPropertyAssignment
// resolves through getShorthandAssignmentValueSymbol, everything else through
// getSymbolAtLocation.
func identifierSymbol(s *State, node *ast.Node) *ast.Symbol {
	var symbol *ast.Symbol
	if node.Parent != nil && ast.IsShorthandPropertyAssignment(node.Parent) {
		symbol = s.Checker.GetShorthandAssignmentValueSymbol(node.Parent)
	} else {
		symbol = s.Checker.GetSymbolAtLocation(node)
	}
	if symbol == nil {
		panic("transformer: identifier has no symbol") // upstream assert
	}
	return symbol
}

// TransformIdentifier ports transformIdentifier.ts transformIdentifier (L111-
// 176) — an identifier in a *use* position.
func TransformIdentifier(s *State, node *ast.Node) luau.Expression {
	// Synthetic nodes don't have parents or symbols, so skip all the
	// symbol-related logic. JSX EntityName functions like getJsxFactoryEntity()
	// return synthetic nodes and transformEntityName eventually ends up here.
	if node.Parent == nil || ast.PositionIsSynthesized(node.Pos()) {
		return luau.ID(node.Text())
	}

	symbol := identifierSymbol(s, node)

	if s.Checker.IsUndefinedSymbol(symbol) {
		return luau.Nil()
	} else if s.Checker.IsArgumentsSymbol(symbol) {
		s.Diags.Add(DiagNoArguments(node))
	} else if isGlobalThisSymbol(symbol) {
		s.Diags.Add(DiagNoGlobalThis(node))
	}

	// Macro hook points (upstream L132-159) — upstream consults the
	// MacroManager here for:
	//   - identifier macros (getIdentifierMacro, e.g. `script`, `Promise`),
	//   - constructor-macro misuse (getFirstConstructSymbol +
	//     getConstructorMacro -> noMacroExtends / noConstructorMacroWithoutNew),
	//   - call-macro misuse outside call position (getCallMacro ->
	//     noIndexWithoutCall).
	// rotor has no macro tables yet. Every value the MacroManager registers is
	// declared by @rbxts/compiler-types, so Phase 2 treats any symbol declared
	// there as a macro and rejects it loudly — in EVERY position, including
	// the call position upstream defers to transformCallExpression, because
	// emitting a macro's name as a plain global would be silently wrong
	// output. full macro tables: Phase 3.
	if isCompilerTypesSymbol(symbol) {
		s.Diags.Add(DiagRotorNotYetSupported(node, "macro `"+node.Text()+"`"))
		return luau.NewNone()
	}

	// `export let` indirection (upstream L161-171): reads of mutable exported
	// symbols compile to `exports.<name>` (exit here so hoisting is never
	// checked for them; consts stay plain locals).
	if vd := symbol.ValueDeclaration; vd != nil &&
		ast.GetSourceFileOfNode(vd) == ast.GetSourceFileOfNode(node) &&
		ast.FindAncestor(vd, func(n *ast.Node) bool {
			return ast.IsModuleDeclaration(n) && !isNamespace(n)
		}) == nil {
		exportAccess := s.GetModuleIDPropertyAccess(symbol)
		if exportAccess != nil && IsSymbolMutable(s, symbol) {
			return exportAccess
		}
	}

	checkIdentifierHoist(s, node, symbol)

	return TransformIdentifierDefined(s, node)
}

// TransformIdentifierDefined ports transformIdentifier.ts
// transformIdentifierDefined (L14-28): an identifier in a *defining* position.
// The symbol is looked up (shorthand-property-assignment aware) so renamed/
// captured variables resolve through SymbolToID; otherwise the identifier's
// own text is used.
func TransformIdentifierDefined(s *State, node *ast.Node) luau.AnyIdentifier {
	symbol := identifierSymbol(s, node)
	if replacement := s.SymbolToID[symbol]; replacement != nil {
		return replacement
	}
	return luau.ID(node.Text())
}

// ---------------------------------------------------------------------------
// checkIdentifierHoist — use-before-declare hoisting (upstream L30-109)
// ---------------------------------------------------------------------------

// getAncestorWhichIsChildOf ports the same-named upstream helper (L30-35):
// climbs from node to the ancestor that is a direct child of parent, or nil
// when parent is not an ancestor of node.
func getAncestorWhichIsChildOf(parent, node *ast.Node) *ast.Node {
	for node.Parent != nil && node.Parent != parent {
		node = node.Parent
	}
	if node.Parent == nil {
		return nil
	}
	return node
}

// getDeclarationFromImport ports the same-named upstream helper (L38-45):
// scans symbol.declarations for one under any import syntax ("for some
// reason, symbol.valueDeclaration doesn't point to imports").
func getDeclarationFromImport(symbol *ast.Symbol) *ast.Node {
	for _, declaration := range symbol.Declarations {
		if ast.FindAncestor(declaration, ast.IsAnyImportSyntax) != nil {
			return declaration
		}
	}
	return nil
}

// checkIdentifierHoist ports transformIdentifier.ts checkIdentifierHoist
// (L47-109): records that a symbol must be pre-declared (`local x` merged at
// the premature reference's statement by createHoistDeclaration) when it is
// referenced lexically before/at its declaring statement within the same
// BlockLike.
func checkIdentifierHoist(s *State, node *ast.Node, symbol *ast.Symbol) {
	if _, decided := s.IsHoisted[symbol]; decided {
		return
	}

	declaration := symbol.ValueDeclaration
	if declaration == nil {
		declaration = getDeclarationFromImport(symbol)
	}

	// parameters cannot be hoisted
	if declaration == nil ||
		ast.FindAncestorKind(declaration, ast.KindParameter) != nil ||
		ast.IsShorthandPropertyAssignment(declaration) {
		return
	}

	// class expressions can self refer
	if ast.IsClassLike(declaration) && isAncestorOf(declaration, node) {
		return
	}

	declarationStatement := ast.FindAncestor(declaration, ast.IsStatement)
	if declarationStatement == nil ||
		ast.IsForStatement(declarationStatement) ||
		ast.IsForOfStatement(declarationStatement) ||
		ast.IsTryStatement(declarationStatement) {
		return
	}

	parent := declarationStatement.Parent
	if parent == nil || !isBlockLike(parent) {
		return
	}

	sibling := getAncestorWhichIsChildOf(parent, node)
	if sibling == nil || !ast.IsStatement(sibling) {
		return
	}

	statements := parent.Statements()
	declarationIdx := indexOfNode(statements, declarationStatement)
	siblingIdx := indexOfNode(statements, sibling)

	if siblingIdx > declarationIdx {
		return
	}

	if siblingIdx == declarationIdx {
		// non-async function declarations, class declarations, and variable
		// statements can self refer
		if (ast.IsFunctionDeclaration(declarationStatement) &&
			!ast.HasSyntacticModifier(declarationStatement, ast.ModifierFlagsAsync)) ||
			ast.IsClassDeclaration(declarationStatement) ||
			(ast.IsVariableStatement(declarationStatement) &&
				ast.FindAncestor(node, func(n *ast.Node) bool {
					return ast.IsStatement(n) || ast.IsFunctionLikeDeclaration(n)
				}) == declarationStatement) {
			return
		}
	}

	s.HoistsByStatement[sibling] = append(s.HoistsByStatement[sibling], node)
	s.IsHoisted[symbol] = true
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// isAncestorOf ports util/traversal.ts isAncestorOf: true when ancestor is
// node or any of node's parents.
func isAncestorOf(ancestor, node *ast.Node) bool {
	for node != nil {
		if ancestor == node {
			return true
		}
		node = node.Parent
	}
	return false
}

// isBlockLike ports typeGuards.ts isBlockLike: SourceFile | Block |
// ModuleBlock | CaseClause | DefaultClause (exactly the kinds tsgo's
// Node.Statements() accepts).
func isBlockLike(node *ast.Node) bool {
	switch node.Kind {
	case ast.KindSourceFile, ast.KindBlock, ast.KindModuleBlock,
		ast.KindCaseClause, ast.KindDefaultClause:
		return true
	}
	return false
}

// isNamespace ports typeGuards.ts isNamespace: a ModuleDeclaration written
// with the `namespace` keyword. Upstream tests NodeFlags.Namespace; tsgo keeps
// the declaring keyword kind on the node instead.
func isNamespace(node *ast.Node) bool {
	return ast.IsModuleDeclaration(node) && node.AsModuleDeclaration().Keyword == ast.KindNamespaceKeyword
}

func indexOfNode(nodes []*ast.Node, node *ast.Node) int {
	for i, n := range nodes {
		if n == node {
			return i
		}
	}
	return -1
}

// isGlobalThisSymbol detects the checker-synthesized `globalThis` symbol.
// Upstream compares against macroManager.getSymbolOrThrow(SYMBOL_NAMES.
// globalThis); tsgo's checker builds that symbol internally (Module-flagged,
// named "globalThis", no declarations) and does not expose it, so rotor
// matches it structurally.
func isGlobalThisSymbol(symbol *ast.Symbol) bool {
	return symbol.Name == "globalThis" &&
		symbol.Flags&ast.SymbolFlagsModule != 0 &&
		len(symbol.Declarations) == 0
}

// isCompilerTypesSymbol reports whether symbol is declared by the
// @rbxts/compiler-types package — upstream's MacroManager builds its macro
// tables exclusively from those declaration files, so this is the Phase 2
// stand-in for "symbol has a macro". full macro tables: Phase 3.
func isCompilerTypesSymbol(symbol *ast.Symbol) bool {
	for _, declaration := range symbol.Declarations {
		if sf := ast.GetSourceFileOfNode(declaration); sf != nil &&
			strings.Contains(sf.FileName(), "node_modules/@rbxts/compiler-types/") {
			return true
		}
	}
	return false
}
