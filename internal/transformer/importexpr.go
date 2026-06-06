package transformer

import (
	"path/filepath"
	"slices"
	"strings"

	"rotor/internal/luau"
	"rotor/internal/rojo"
	"rotor/tsgo/ast"
)

// nodeModules ports Shared/constants.ts NODE_MODULES.
const nodeModules = "node_modules"

// getSourceFileFromModuleSpecifier ports
// util/getSourceFileFromModuleSpecifier.ts (L4-33): the program SourceFile a
// module specifier resolves to, or nil.
func getSourceFileFromModuleSpecifier(s *State, moduleSpecifier *ast.Node) *ast.SourceFile {
	symbol := s.Checker.GetSymbolAtLocation(moduleSpecifier)
	if symbol == nil {
		symbol = s.Checker.ResolveExternalModuleName(moduleSpecifier)
	}
	if symbol != nil {
		declaration := symbol.ValueDeclaration

		if declaration != nil && ast.IsModuleDeclaration(declaration) && ast.IsStringLiteralLike(declaration.Name()) {
			// Ambient module declaration: chase the REAL file through the
			// program's resolution cache.
			sourceFile := ast.GetSourceFileOfNode(moduleSpecifier)
			mode := s.Program.GetModeForUsageLocation(sourceFile, declaration.Name())
			resolvedModule := s.Program.GetResolvedModule(sourceFile, declaration.Name().Text(), mode)
			if resolvedModule.IsResolved() {
				return s.Program.GetSourceFile(resolvedModule.ResolvedFileName)
			}
		}

		if declaration != nil && ast.IsSourceFile(declaration) {
			return declaration.AsSourceFile()
		}
	}

	// Upstream falls back to a raw ts.resolveModuleName for $getModuleTree of
	// modules never referenced by a regular import (L27-32) — deferred with
	// the $getModuleTree macro (digest §3.2: "only needed for $getModuleTree
	// of never-imported modules; defer").
	return nil
}

// isInsideNodeModules reimplements strada's ts.isInsideNodeModules (no public
// tsgo ast helper, digest §9.2): any path component equals "node_modules".
func isInsideNodeModules(filePath string) bool {
	for _, part := range strings.FieldsFunc(filepath.ToSlash(filePath), func(r rune) bool { return r == '/' }) {
		if part == nodeModules {
			return true
		}
	}
	return false
}

// relToProject renders a path relative to the project dir for noRojoData
// diagnostics (upstream path.relative(state.data.projectPath, ...)).
func relToProject(s *State, target string) string {
	rel, err := filepath.Rel(s.Rojo.ProjectPath, target)
	if err != nil {
		return target
	}
	return rel
}

// getAbsoluteImport ports createImportExpression.ts getAbsoluteImport
// (L15-24): [game:GetService("<p0>"), "p1", "p2", ...].
func getAbsoluteImport(moduleRbxPath rojo.RbxPath) []luau.Expression {
	serviceName := moduleRbxPath[0] // upstream assert(serviceName)
	pathExpressions := []luau.Expression{createGetService(serviceName)}
	for _, part := range moduleRbxPath[1:] {
		pathExpressions = append(pathExpressions, luau.Str(part))
	}
	return pathExpressions
}

// getRelativeImport ports createImportExpression.ts getRelativeImport
// (L26-47): leading RbxPathParent segments fold into ONE
// `script.Parent.Parent...` root expression; the remaining names follow as
// string arguments.
func getRelativeImport(sourceRbxPath, moduleRbxPath rojo.RbxPath) []luau.Expression {
	relativePath := rojo.Relative(sourceRbxPath, moduleRbxPath)

	var parents []string
	i := 0
	for i < len(relativePath) && relativePath[i].Parent {
		parents = append(parents, parentField)
		i++
	}

	pathExpressions := []luau.Expression{propertyAccessExpressionChain(luau.GlobalID("script"), parents)}
	for ; i < len(relativePath); i++ {
		// upstream assert(typeof pathPart === "string"): Relative never
		// yields a Parent after a name segment.
		pathExpressions = append(pathExpressions, luau.Str(relativePath[i].Name))
	}
	return pathExpressions
}

// validateModule ports createImportExpression.ts validateModule (L49-59): the
// scope's node_modules directory must path-equal one of the configured
// typeRoots.
func validateModule(s *State, scope string) bool {
	scopedModules := filepath.Clean(filepath.Join(s.Rojo.NodeModulesPath, scope))
	for _, typeRoot := range s.Rojo.TypeRoots {
		if scopedModules == filepath.Clean(filepath.FromSlash(typeRoot)) {
			return true
		}
	}
	return false
}

// findRelativeRbxPath ports createImportExpression.ts findRelativeRbxPath
// (L61-68): first pkg resolver that covers the path wins.
func findRelativeRbxPath(moduleOutPath string, pkgRojoResolvers []*rojo.RojoResolver) (rojo.RbxPath, bool) {
	for _, pkgRojoResolver := range pkgRojoResolvers {
		if relativeRbxPath, ok := pkgRojoResolver.GetRbxPathFromFilePath(moduleOutPath); ok {
			return relativeRbxPath, true
		}
	}
	return nil, false
}

// getNodeModulesImportParts ports createImportExpression.ts
// getNodeModulesImportParts (L70-134).
func getNodeModulesImportParts(s *State, sourceFile *ast.SourceFile, moduleSpecifier *ast.Node, moduleOutPath string) []luau.Expression {
	rel, err := filepath.Rel(s.Rojo.NodeModulesPath, moduleOutPath)
	if err != nil || rel == "" {
		panic("transformer: getNodeModulesImportParts: no module scope") // upstream assert(moduleScope)
	}
	moduleScope := strings.Split(rel, string(filepath.Separator))[0]

	if !strings.HasPrefix(moduleScope, "@") {
		s.Diags.Add(DiagNoUnscopedModule(moduleSpecifier))
		return []luau.Expression{luau.NewNone()}
	}

	if !validateModule(s, moduleScope) {
		s.Diags.Add(DiagNoInvalidModule(moduleSpecifier))
		return []luau.Expression{luau.NewNone()}
	}

	if s.ProjectType == ProjectTypePackage {
		relativeRbxPath, ok := findRelativeRbxPath(moduleOutPath, s.Rojo.PkgRojoResolvers)
		if !ok {
			s.Diags.Add(DiagNoRojoData(moduleSpecifier, relToProject(s, moduleOutPath), true))
			return []luau.Expression{luau.NewNone()}
		}

		moduleName := relativeRbxPath[0] // upstream assert(moduleName)

		return []luau.Expression{
			propertyAccessExpressionChain(
				luau.NewCall(s.RuntimeLib(moduleSpecifier.Parent, "getModule"), luau.NewList[luau.Expression](
					luau.GlobalID("script"),
					luau.Str(moduleScope),
					luau.Str(moduleName),
				)),
				relativeRbxPath[1:],
			),
		}
	}

	moduleRbxPath, ok := s.Rojo.Resolver.GetRbxPathFromFilePath(moduleOutPath)
	if !ok {
		s.Diags.Add(DiagNoRojoData(moduleSpecifier, relToProject(s, moduleOutPath), true))
		return []luau.Expression{luau.NewNone()}
	}

	indexOfScope := slices.Index(moduleRbxPath, moduleScope)
	if indexOfScope <= 0 || moduleRbxPath[indexOfScope-1] != nodeModules {
		s.Diags.Add(DiagNoPackageImportWithoutScope(moduleSpecifier, relToProject(s, moduleOutPath), moduleRbxPath))
		return []luau.Expression{luau.NewNone()}
	}

	return getProjectImportParts(s, sourceFile, moduleSpecifier, moduleOutPath, moduleRbxPath)
}

// getProjectImportParts ports createImportExpression.ts getProjectImportParts
// (L136-182).
func getProjectImportParts(s *State, sourceFile *ast.SourceFile, moduleSpecifier *ast.Node, moduleOutPath string, moduleRbxPath rojo.RbxPath) []luau.Expression {
	moduleRbxType := s.Rojo.Resolver.GetRbxTypeFromFilePath(moduleOutPath)
	if moduleRbxType == rojo.RbxTypeScript || moduleRbxType == rojo.RbxTypeLocalScript {
		s.Diags.Add(DiagNoNonModuleImport(moduleSpecifier))
		return []luau.Expression{luau.NewNone()}
	}

	sourceOutPath := s.Rojo.PathTranslator.GetOutputPath(sourceFile.FileName())
	sourceRbxPath, ok := s.Rojo.Resolver.GetRbxPathFromFilePath(sourceOutPath)
	if !ok {
		s.Diags.Add(DiagNoRojoData(sourceFile.AsNode(), relToProject(s, sourceOutPath), false))
		return []luau.Expression{luau.NewNone()}
	}

	if s.ProjectType == ProjectTypeGame {
		// In the case of `import("")`, don't do the network type check as the
		// call may be guarded by runtime RunService checks (upstream comment).
		if !ast.IsImportCall(moduleSpecifier.Parent) &&
			s.Rojo.Resolver.GetNetworkType(moduleRbxPath) == rojo.NetworkTypeServer &&
			s.Rojo.Resolver.GetNetworkType(sourceRbxPath) != rojo.NetworkTypeServer {
			s.Diags.Add(DiagNoServerImport(moduleSpecifier))
			return []luau.Expression{luau.NewNone()}
		}

		fileRelation := s.Rojo.Resolver.GetFileRelation(sourceRbxPath, moduleRbxPath)
		switch fileRelation {
		case rojo.FileRelationOutToOut, rojo.FileRelationInToOut:
			return getAbsoluteImport(moduleRbxPath)
		case rojo.FileRelationInToIn:
			return getRelativeImport(sourceRbxPath, moduleRbxPath)
		default:
			s.Diags.Add(DiagNoIsolatedImport(moduleSpecifier))
			return []luau.Expression{luau.NewNone()}
		}
	}

	return getRelativeImport(sourceRbxPath, moduleRbxPath)
}

// getImportParts ports createImportExpression.ts getImportParts (L184-210):
// resolve the specifier to a file, translate to its output location, and pick
// the node_modules or project pipeline. Every error path adds its diagnostic
// and returns [none] (the compile bails on the diagnostic before rendering).
func getImportParts(s *State, sourceFile *ast.SourceFile, moduleSpecifier *ast.Node) []luau.Expression {
	moduleFile := getSourceFileFromModuleSpecifier(s, moduleSpecifier)
	if moduleFile == nil {
		s.Diags.Add(DiagNoModuleSpecifierFile(moduleSpecifier))
		return []luau.Expression{luau.NewNone()}
	}

	// TODO(phase-3 symlinks): upstream consults state.guessVirtualPath first
	// (TransformState.ts:367-385, reverse-symlink lookup over the program's
	// symlink cache for pnpm-style node_modules); its `|| fileName` fallback
	// is the unconditional behavior here until tsgo's symlinks package is
	// wired (digest §3.1.2: "Port later; fallback is safe default").
	virtualPath := moduleFile.FileName()

	if isInsideNodeModules(virtualPath) {
		mappedPath := virtualPath
		key := rojo.CanonicalFileName(virtualPath, s.Rojo.UseCaseSensitiveFileNames)
		if mapped, ok := s.Rojo.NodeModulesPathMapping[key]; ok {
			mappedPath = mapped
		}
		moduleOutPath := s.Rojo.PathTranslator.GetImportPath(mappedPath, true /* isNodeModule */)
		return getNodeModulesImportParts(s, sourceFile, moduleSpecifier, moduleOutPath)
	}

	moduleOutPath := s.Rojo.PathTranslator.GetImportPath(virtualPath, false)
	moduleRbxPath, ok := s.Rojo.Resolver.GetRbxPathFromFilePath(moduleOutPath)
	if !ok {
		s.Diags.Add(DiagNoRojoData(moduleSpecifier, relToProject(s, moduleOutPath), false))
		return []luau.Expression{luau.NewNone()}
	}
	return getProjectImportParts(s, sourceFile, moduleSpecifier, moduleOutPath, moduleRbxPath)
}

// createImportExpression ports createImportExpression.ts (L212-220):
// `TS.import(script, <root expr>, "<name>"...)`. The state.TS call sets
// UsesRuntimeLib; WaitForChild semantics live inside RuntimeLib's TS.import
// at runtime — the emitted AST carries plain strings.
func createImportExpression(s *State, sourceFile *ast.SourceFile, moduleSpecifier *ast.Node) luau.IndexableExpression {
	parts := getImportParts(s, sourceFile, moduleSpecifier)
	args := luau.NewList[luau.Expression](luau.GlobalID("script"))
	for _, part := range parts {
		args.Push(part)
	}
	return luau.NewCall(s.RuntimeLib(moduleSpecifier.Parent, "import"), args)
}

// transformImportExpression ports
// nodes/expressions/transformImportExpression.ts (L8-30) — dynamic
// `import("x")`:
// `TS.Promise.new(function(resolve) resolve(TS.import(script, ...)) end)`.
func transformImportExpression(s *State, node *ast.Node) luau.Expression {
	var moduleSpecifier *ast.Node
	if arguments := node.Arguments(); len(arguments) > 0 {
		moduleSpecifier = arguments[0]
	}

	if moduleSpecifier == nil || !ast.IsStringLiteral(moduleSpecifier) {
		s.Diags.Add(DiagNoNonStringModuleSpecifier(node))
		return luau.NewNone()
	}

	importExpression := createImportExpression(s, ast.GetSourceFileOfNode(node), moduleSpecifier)
	resolveID := luau.ID("resolve")

	return luau.NewCall(
		luau.NewPropertyAccess(s.RuntimeLib(node, "Promise"), "new"),
		luau.NewList[luau.Expression](luau.NewFunctionExpression(
			luau.NewList[luau.AnyIdentifier](resolveID),
			false,
			luau.NewList[luau.Statement](luau.NewCallStatement(
				luau.NewCall(resolveID, luau.NewList[luau.Expression](importExpression)),
			)),
		)),
	)
}
