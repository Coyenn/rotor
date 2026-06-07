package transformer

import (
	"rotor/internal/luau"
	"rotor/tsgo/ast"
	"rotor/tsgo/checker"
)

// This file ports statements/transformModuleDeclaration.ts.

// isDeclarationOfNamespace ports transformModuleDeclaration.ts
// isDeclarationOfNamespace (L16-30) — the merge filter: a declaration counts
// toward "namespace value definitions" iff it has no `declare` modifier and is
// an instantiated module, a function declaration WITH a body, or a class
// declaration. (Namespace+interface merging is fine; namespace+namespace,
// namespace+function-with-body, and namespace+class all trigger the ban.)
func isDeclarationOfNamespace(declaration *ast.Node) bool {
	if ast.HasSyntacticModifier(declaration, ast.ModifierFlagsAmbient) {
		return false
	}

	if ast.IsModuleDeclaration(declaration) && ast.IsInstantiatedModule(declaration, false) {
		return true
	} else if ast.IsFunctionDeclaration(declaration) && declaration.Body() != nil {
		return true
	} else if ast.IsClassDeclaration(declaration) {
		return true
	}
	return false
}

// getValueDeclarationStatement ports transformModuleDeclaration.ts
// getValueDeclarationStatement (L32-44): for an export symbol, the STATEMENT
// that declares its value — skipping function overload signatures, type
// aliases, interfaces, and `declare`d statements.
func getValueDeclarationStatement(symbol *ast.Symbol) *ast.Node {
	for _, declaration := range symbol.Declarations {
		statement := ast.FindAncestor(declaration, ast.IsStatement)
		if statement != nil {
			if ast.IsFunctionDeclaration(statement) && statement.Body() == nil {
				continue
			}
			if ast.IsTypeAliasDeclaration(statement) {
				continue
			}
			if ast.IsInterfaceDeclaration(statement) {
				continue
			}
			if ast.HasSyntacticModifier(statement, ast.ModifierFlagsAmbient) {
				continue
			}
			return statement
		}
	}
	return nil
}

// transformNamespace ports transformModuleDeclaration.ts transformNamespace
// (L46-122): `local X = {}` (or `X = {}` when hoisted) + a do-block. With
// value exports the do-block opens with `local _container = X`; non-mutable
// value exports are appended after their declaring statement by the
// statement-list ExportInfo mapping, while mutable ones (`export let`) are
// EXCLUDED from the mapping — SetModuleIDBySymbol(containerId) routes their
// declarations/reads/writes through `_container.<name>` via the existing
// export-let indirection. body is a ModuleBlock, or a nested ModuleDeclaration
// for dotted `namespace A.B` (recurse + `_container.B = B`).
func transformNamespace(s *State, name *ast.Node, body *ast.Node) *luau.List[luau.Statement] {
	symbol := s.Checker.GetSymbolAtLocation(name)
	if symbol == nil {
		panic("transformer: transformNamespace: no symbol") // upstream assert
	}

	ValidateIdentifier(s, name)

	nameExp := TransformIdentifierDefined(s, name)

	statements := luau.NewList[luau.Statement]()
	doStatements := luau.NewList[luau.Statement]()

	containerID := luau.TempID("container")
	s.SetModuleIDBySymbol(symbol, containerID)

	if s.IsHoisted[symbol] {
		statements.Push(luau.NewAssignment(nameExp, "=", luau.NewMap(luau.NewList[*luau.MapField]())))
	} else {
		statements.Push(luau.NewVariableDeclaration(nameExp, luau.NewMap(luau.NewList[*luau.MapField]())))
	}

	moduleExports := s.GetModuleExports(symbol)
	if len(moduleExports) > 0 {
		doStatements.Push(luau.NewVariableDeclaration(containerID, nameExp))
	}

	if ast.IsModuleBlock(body) {
		exportsMap := map[*ast.Node][]string{}
		if len(moduleExports) > 0 {
			for _, exportSymbol := range moduleExports {
				originalSymbol := checker.SkipAlias(exportSymbol, s.Checker)
				if isSymbolOfValue(originalSymbol) && !IsSymbolMutable(s, originalSymbol) {
					if valueDeclarationStatement := getValueDeclarationStatement(exportSymbol); valueDeclarationStatement != nil {
						exportsMap[valueDeclarationStatement] = append(exportsMap[valueDeclarationStatement], exportSymbol.Name)
					}
				}
			}
		}
		doStatements.PushList(TransformStatementList(s, body, body.AsModuleBlock().Statements.Nodes, &ExportInfo{
			ID:      containerID,
			Mapping: exportsMap,
		}))
	} else {
		// dotted `namespace A.B { ... }`: tsgo parses the tail as a nested
		// ModuleDeclaration (with an implicit reparsed export modifier)
		doStatements.PushList(transformNamespace(s, body.Name(), body.Body()))
		doStatements.Push(luau.NewAssignment(
			luau.NewPropertyAccess(containerID, body.Name().Text()),
			"=",
			TransformIdentifierDefined(s, body.Name()),
		))
	}

	statements.Push(luau.NewDo(doStatements))

	return statements
}

// transformModuleDeclaration ports transformModuleDeclaration.ts
// transformModuleDeclaration (L124-146). Type-only namespaces emit nothing;
// merging is banned (reported once per symbol); `declare module "X"` and
// `declare namespace` are filtered by the dispatch-level declare skip.
func transformModuleDeclaration(s *State, node *ast.Node) *luau.List[luau.Statement] {
	// type-only namespace
	if !ast.IsInstantiatedModule(node, false) {
		return luau.NewList[luau.Statement]()
	}

	// disallow merging
	symbol := s.Checker.GetSymbolAtLocation(node.Name())
	if symbol != nil && hasMultipleDefinitions(symbol, isDeclarationOfNamespace) {
		AddDiagnosticWithCache(s.Diags, symbol, DiagNoNamespaceMerging(node),
			s.Multi.IsReportedByMultipleDefinitionsCache)
		return luau.NewList[luau.Statement]()
	}

	// ts.StringLiteral is only in the case of `declare module "X"`? Should be filtered out above
	if ast.IsStringLiteral(node.Name()) {
		panic("transformer: transformModuleDeclaration: string literal name") // upstream assert
	}
	body := node.Body()
	if body == nil || ast.IsIdentifier(body) {
		panic("transformer: transformModuleDeclaration: unexpected body") // upstream assert
	}
	return transformNamespace(s, node.Name(), body)
}
