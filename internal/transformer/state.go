package transformer

import (
	"rotor/internal/luau"
	"rotor/internal/rojo"
	"rotor/tsgo/ast"
	"rotor/tsgo/checker"
	"rotor/tsgo/compiler"
)

// ProjectType ports Shared/constants.ts ProjectType.
type ProjectType string

const (
	ProjectTypeGame    ProjectType = "game"
	ProjectTypeModel   ProjectType = "model"
	ProjectTypePackage ProjectType = "package"
)

// MultiState ports classes/MultiTransformState.ts: caches that live for one
// whole compilation step, shared across every file of that pass (recreated on
// each watch rebuild). Pure cache container, no methods.
type MultiState struct {
	IsMethodCache                        map[*ast.Symbol]bool
	IsDefinedAsLetCache                  map[*ast.Symbol]bool
	IsReportedByNoAnyCache               map[*ast.Symbol]bool
	IsReportedByMultipleDefinitionsCache map[*ast.Symbol]bool
	GetModuleExportsCache                map[*ast.Symbol][]*ast.Symbol
	GetModuleExportsAliasMapCache        map[*ast.Symbol]map[*ast.Symbol]string

	// macroManager is the upstream TransformServices analog: ONE MacroManager
	// per compilation pass, shared across every file's State (upstream
	// constructs it next to the MultiTransformState in compileFiles.ts and
	// threads it through `services`). Built by NewState from the first
	// checker-bearing State; see State.Macros.
	macroManager *MacroManager
}

// RojoContext carries the project-level Rojo data the reference TransformState
// receives as constructor arguments (compileFiles.ts:160-174): the resolver
// over the project's Rojo config, the input->output PathTranslator, the
// RuntimeLib rbxPath computed once per compile (compileFiles.ts:86-98), and
// the project path that noRojoData renders file paths relative to. Computed
// once per CompileProject pass and shared by every file's State.
type RojoContext struct {
	Resolver       *rojo.RojoResolver
	PathTranslator *rojo.PathTranslator
	// RuntimeLibRbxPath is nil for Package projects (upstream undefined),
	// selecting the `_G[script]` runtime-lib access form.
	RuntimeLibRbxPath rojo.RbxPath
	// ProjectPath is upstream data.projectPath (the tsconfig.json directory).
	ProjectPath string

	// Import-resolution members (createImportExpression pipeline, digest §3):

	// PkgRojoResolvers holds one RojoResolver.synthetic per typeRoot
	// (compileFiles.ts:77) — Package projects resolve node_modules imports
	// through these instead of the project resolver.
	PkgRojoResolvers []*rojo.RojoResolver
	// NodeModulesPathMapping maps the canonical types-entry path (.d.ts) of
	// each typeRoot package to its shipped main (.lua) path
	// (createNodeModulesPathMapping.ts). Keys are canonicalized via
	// rojo.CanonicalFileName with UseCaseSensitiveFileNames.
	NodeModulesPathMapping map[string]string
	// NodeModulesPath is upstream data.nodeModulesPath:
	// <package.json dir>/node_modules (createProjectData.ts:31).
	NodeModulesPath string
	// TypeRoots is compilerOptions.typeRoots as tsgo resolved them (absolute,
	// slash-separated); validateModule checks npm scopes against these.
	TypeRoots []string
	// UseCaseSensitiveFileNames feeds the canonical-file-name lookups
	// (Shared/util/getCanonicalFileName.ts).
	UseCaseSensitiveFileNames bool
}

// NewMultiState returns an empty compilation-step cache container.
func NewMultiState() *MultiState {
	return &MultiState{
		IsMethodCache:                        make(map[*ast.Symbol]bool),
		IsDefinedAsLetCache:                  make(map[*ast.Symbol]bool),
		IsReportedByNoAnyCache:               make(map[*ast.Symbol]bool),
		IsReportedByMultipleDefinitionsCache: make(map[*ast.Symbol]bool),
		GetModuleExportsCache:                make(map[*ast.Symbol][]*ast.Symbol),
		GetModuleExportsAliasMapCache:        make(map[*ast.Symbol]map[*ast.Symbol]string),
	}
}

// State ports TSTransformer/classes/TransformState.ts: the per-source-file
// transform state. It owns prereq-statement stacking, hoisting bookkeeping,
// runtime-lib usage tracking, type caching, and module-export id mapping.
//
// Phase 3/4: emit-resolver-dependent state is absent for now — upstream's
// `resolver = typeChecker.getEmitResolver(sourceFile)` is consumed only by
// JSX factory entity lookups (getJsxFactoryEntity/getJsxFragmentFactoryEntity)
// and import/export elision (isReferencedAliasDeclaration).
// Phase 3: classIdentifierMap, classElementToObjectKeyMap (class transforms)
// and the tryUsesStack (try/catch flow-control tunneling) land with their
// consumers.
type State struct {
	Program    *compiler.Program // may be nil in mechanics tests
	Checker    *checker.Checker  // may be nil in mechanics tests
	SourceFile *ast.SourceFile
	Diags      *DiagService
	Multi      *MultiState

	// Rojo is the project-level Rojo context (nil in mechanics tests and in
	// any State that never reaches runtime-lib emission). Upstream threads the
	// equivalent fields through the TransformState constructor; rotor keeps
	// NewState's signature stable for the test fleet and installs the context
	// via SetRojoContext.
	Rojo *RojoContext

	// ProjectType + IsInReplicatedFirst gate the runtimeLibUsedInReplicatedFirst
	// warning in RuntimeLib. SetRojoContext computes IsInReplicatedFirst per
	// the upstream constructor (TransformState.ts:62-65:
	// pathTranslator.getOutputPath + rojoResolver.getRbxPathFromFilePath,
	// rbxPath[0] === "ReplicatedFirst"); without a Rojo context both stay
	// zero-valued and the warning never fires.
	ProjectType         ProjectType
	IsInReplicatedFirst bool

	// SourceFileText is the full source text including leading trivia
	// (upstream sourceFile.getFullText()); used for comment extraction and
	// raw-text literal slicing.
	SourceFileText string

	// UsesRuntimeLib is set ONLY by RuntimeLib (upstream TS() is the only
	// assignment in the entire repo); gates runtime-lib import emission.
	UsesRuntimeLib bool

	// LogTruthyChanges plumbs the `logTruthyChanges` project option into
	// createTruthinessChecks warnings (default off).
	LogTruthyChanges bool

	// HasExportEquals is set by transformExportAssignment when `export = x`
	// is seen; changes export emission shape.
	HasExportEquals bool
	// HasExportFrom is set by transformExportDeclaration when
	// `export ... from "..."` is seen; forces the `local exports = {}` form.
	HasExportFrom bool

	// prereqStack ports prereqStatementsStack — THE core mechanism: a stack of
	// statement lists that prerequisite statements are appended onto.
	prereqStack []*luau.List[luau.Statement]

	// getTypeCache memoizes GetType by the ORIGINAL node pointer (not the
	// SkipUpwards result).
	getTypeCache map[*ast.Node]*checker.Type

	// HoistsByStatement maps a ts.Statement (or ts.CaseClause) to the
	// ts.Identifier nodes needing a `local x` hoist emitted just before that
	// statement. NOTE: these are TS identifiers, not luau nodes —
	// createHoistDeclaration runs transformIdentifierDefined on them later.
	HoistsByStatement map[*ast.Node][]*ast.Node
	// IsHoisted memoizes per-symbol hoist decisions (upstream reads
	// `.get(symbol) !== undefined`; once decided, never reconsidered).
	IsHoisted map[*ast.Symbol]bool

	// SymbolToID maps a symbol to its replacement temp id (upstream
	// symbolToIdMap, used by transformIdentifierDefined for renamed/captured
	// vars).
	SymbolToID map[*ast.Symbol]*luau.TemporaryIdentifier

	// moduleIDBySymbol maps a module symbol to the luau id holding that
	// module's exports table (source file symbol -> `exports`; namespaces ->
	// their container id).
	moduleIDBySymbol map[*ast.Symbol]luau.AnyIdentifier
}

// NewState constructs the per-file transform state. program/chk/sourceFile may
// be nil for checker-free prereq-mechanics tests (see NewTestState).
func NewState(program *compiler.Program, chk *checker.Checker, sourceFile *ast.SourceFile, diags *DiagService, multi *MultiState) *State {
	s := &State{
		Program:           program,
		Checker:           chk,
		SourceFile:        sourceFile,
		Diags:             diags,
		Multi:             multi,
		getTypeCache:      make(map[*ast.Node]*checker.Type),
		HoistsByStatement: make(map[*ast.Node][]*ast.Node),
		IsHoisted:         make(map[*ast.Symbol]bool),
		SymbolToID:        make(map[*ast.Symbol]*luau.TemporaryIdentifier),
		moduleIDBySymbol:  make(map[*ast.Symbol]luau.AnyIdentifier),
	}
	if sourceFile != nil {
		s.SourceFileText = sourceFile.Text()
	}
	// One MacroManager per compilation pass (upstream constructs it once per
	// Program); the first checker-bearing State builds it.
	if multi.macroManager == nil && chk != nil {
		multi.macroManager = NewMacroManager(chk)
	}
	return s
}

// SetRojoContext installs the project-level Rojo context and project type,
// then computes IsInReplicatedFirst exactly as the upstream TransformState
// constructor does (TransformState.ts:62-65). Call before TransformSourceFile;
// upstream takes these as constructor arguments, rotor keeps NewState's
// signature stable for existing call sites.
func (s *State) SetRojoContext(rc *RojoContext, projectType ProjectType) {
	s.Rojo = rc
	s.ProjectType = projectType
	if rc != nil && s.SourceFile != nil {
		sourceOutPath := rc.PathTranslator.GetOutputPath(s.SourceFile.FileName())
		rbxPath, ok := rc.Resolver.GetRbxPathFromFilePath(sourceOutPath)
		s.IsInReplicatedFirst = ok && len(rbxPath) > 0 && rbxPath[0] == "ReplicatedFirst"
	}
}

// NewTestState returns a state with nil program/checker for prereq-mechanics
// tests that never touch types.
func NewTestState() *State {
	return NewState(nil, nil, nil, NewDiagService(), NewMultiState())
}

// ---------------------------------------------------------------------------
// Prereq statement stack (upstream lines 99-178)
// ---------------------------------------------------------------------------

// Prereq appends a prerequisite statement to the top of the prereq stack
// (upstream prereq). Calling with an empty stack panics, matching the
// upstream JS crash (`undefined.push`); every transformStatement call site is
// wrapped in Capture, so a list is always present during transformation.
func (s *State) Prereq(stmt luau.Statement) {
	s.prereqStack[len(s.prereqStack)-1].Push(stmt)
}

// PrereqList splices an entire statement list onto the top of the prereq
// stack (upstream prereqList; the source list must not be reused).
func (s *State) PrereqList(list *luau.List[luau.Statement]) {
	s.prereqStack[len(s.prereqStack)-1].PushList(list)
}

func (s *State) pushPrereqStack() *luau.List[luau.Statement] {
	list := luau.NewList[luau.Statement]()
	s.prereqStack = append(s.prereqStack, list)
	return list
}

func (s *State) popPrereqStack() *luau.List[luau.Statement] {
	n := len(s.prereqStack)
	if n == 0 {
		panic("transformer: popPrereqStack on empty stack") // upstream assert
	}
	top := s.prereqStack[n-1]
	s.prereqStack = s.prereqStack[:n-1]
	return top
}

// CaptureStatements runs cb with a fresh prereq list on the stack and returns
// the statements it produced (upstream capturePrereqs).
func (s *State) CaptureStatements(cb func()) *luau.List[luau.Statement] {
	depth := len(s.prereqStack)
	list := s.pushPrereqStack()
	// DIVERGENCE: upstream JS has no try/finally — an exception aborts the
	// whole compile, so stack hygiene doesn't matter there. In Go a recovered
	// panic (e.g. NoPrereqs' assert caught by a test) would leave the stack
	// dirty, so restore the entry depth on the way out. This is a no-op on
	// the normal path, which pops exactly the list pushed above.
	defer func() {
		if len(s.prereqStack) > depth {
			s.prereqStack = s.prereqStack[:depth]
		}
	}()
	cb()
	s.popPrereqStack()
	return list
}

// Capture returns the expression produced by cb along with its prerequisite
// statements (upstream capture<T>; Go methods cannot be generic, so this is
// specialized to the dominant luau.Expression case).
func (s *State) Capture(cb func() luau.Expression) (luau.Expression, *luau.List[luau.Statement]) {
	var value luau.Expression
	prereqs := s.CaptureStatements(func() { value = cb() })
	return value, prereqs
}

// NoPrereqs runs cb and asserts it created no prerequisite statements
// (upstream noPrereqs; the assert is a panic, as upstream).
func (s *State) NoPrereqs(cb func() luau.Expression) luau.Expression {
	var expression luau.Expression
	statements := s.CaptureStatements(func() { expression = cb() })
	if statements.IsNonEmpty() {
		panic("transformer: NoPrereqs callback created prerequisite statements")
	}
	return expression
}

// ---------------------------------------------------------------------------
// pushToVar family (upstream lines 267-306)
// ---------------------------------------------------------------------------

// PushToVar declares and defines a new temp: emits `local <temp> = <expr>` as
// a prereq and returns the temp (upstream pushToVar). expr may be nil to
// pre-declare a temp without a value (`local _temp`). The temp's name hint is
// nameHint if non-empty, else derived via ValueToIdStr — exactly upstream's
// `name || (expression && valueToIdStr(expression))` ("" is falsy in JS).
func (s *State) PushToVar(expr luau.Expression, nameHint string) *luau.TemporaryIdentifier {
	name := nameHint
	if name == "" && expr != nil {
		name = ValueToIdStr(expr)
	}
	temp := luau.TempID(name)
	var right luau.NodeOrList
	if expr != nil {
		right = expr
	}
	s.Prereq(luau.NewVariableDeclaration(temp, right))
	return temp
}

// PushToVarIfComplex returns expr unchanged when it is simple (upstream
// luau.isSimple: Identifier, TemporaryIdentifier, NilLiteral, TrueLiteral,
// FalseLiteral, NumberLiteral, StringLiteral), else pushes it to a temp.
func (s *State) PushToVarIfComplex(expr luau.Expression, nameHint string) luau.Expression {
	if luau.IsSimple(expr) {
		return expr
	}
	return s.PushToVar(expr, nameHint)
}

// PushToVarIfNonID returns expr unchanged when it is already an identifier
// (upstream luau.isAnyIdentifier: Identifier | TemporaryIdentifier only),
// else pushes it to a temp.
func (s *State) PushToVarIfNonID(expr luau.Expression, nameHint string) luau.AnyIdentifier {
	if id, ok := expr.(luau.AnyIdentifier); ok {
		return id
	}
	return s.PushToVar(expr, nameHint)
}

// ---------------------------------------------------------------------------
// getType (upstream lines 183-186)
// ---------------------------------------------------------------------------

// GetType returns the type at SkipUpwards(node), memoized by the original
// node pointer (upstream getType). The nil-recompute mirrors upstream
// getOrSetDefault, which re-computes when the stored value is undefined.
func (s *State) GetType(node *ast.Node) *checker.Type {
	if t := s.getTypeCache[node]; t != nil {
		return t
	}
	t := s.Checker.GetTypeAtLocation(SkipUpwards(node))
	s.getTypeCache[node] = t
	return t
}

// ---------------------------------------------------------------------------
// RuntimeLib (upstream TS(), lines 188-197)
// ---------------------------------------------------------------------------

// RuntimeLib ports upstream TS(node, name): returns `TS.<name>` and flips
// UsesRuntimeLib — this is the SOLE place UsesRuntimeLib is set. The node
// parameter exists solely for the warning's source location and may be nil
// outside Game/ReplicatedFirst files.
func (s *State) RuntimeLib(node *ast.Node, name string) luau.IndexableExpression {
	s.UsesRuntimeLib = true
	if s.ProjectType == ProjectTypeGame && s.IsInReplicatedFirst {
		// Emitted once per call, not deduped (upstream).
		s.Diags.Add(DiagRuntimeLibUsedInReplicatedFirst(node))
	}
	return luau.GlobalProperty("TS", name)
}

// ---------------------------------------------------------------------------
// Emit resolver (upstream state.resolver, TransformState.ts:61)
// ---------------------------------------------------------------------------

// EmitResolver returns the checker's emit resolver, the upstream
// `state.resolver = typeChecker.getEmitResolver(sourceFile)`. Its only
// consumer in this phase is IsReferencedAliasDeclaration (import/export
// elision, digest §1.4).
//
// CHECKER-IDENTITY CONTRACT: `aliasSymbolLinks.referenced` marks are stored on
// the checker INSTANCE that semantically checked the file (markAliasReferenced,
// tsgo checker.go:28500). tsgo's built-in pool spreads files round-robin over
// up to 4 checkers and GetSemanticDiagnostics uses the file-associated one, so
// s.Checker MUST be that same instance or elision queries silently return
// false. rotor guarantees this by pinning the pool to a single checker
// (compilerOptions.Checkers = 1 in compile.newProjectProgram) and running
// program.GetSemanticDiagnostics(ctx, file) before transforming each file —
// proven by TestCompileProjectImports's elision cases.
func (s *State) EmitResolver() *checker.EmitResolver {
	return s.Checker.GetEmitResolver()
}

// ---------------------------------------------------------------------------
// MacroManager access (upstream services.macroManager)
// ---------------------------------------------------------------------------

// Macros returns the pass-wide MacroManager (upstream
// state.services.macroManager), creating an empty one lazily for
// checker-free mechanics states.
func (s *State) Macros() *MacroManager {
	if s.Multi.macroManager == nil {
		s.Multi.macroManager = NewMacroManager(s.Checker)
	}
	return s.Multi.macroManager
}

// AmbientSymbol resolves a global ambient symbol by name through the
// MacroManager's SYMBOL_NAMES registry (upstream
// macroManager.getSymbolOrThrow; rotor returns nil instead of throwing so
// checker-light test projects keep working — callers nil-guard).
func (s *State) AmbientSymbol(name string) *ast.Symbol {
	return s.Macros().Symbol(name)
}

// LuaTupleNominalSymbol resolves the `_nominal_LuaTuple` property symbol from
// the ambient LuaTuple<T> type alias (see MacroManager.LuaTupleNominalSymbol).
func (s *State) LuaTupleNominalSymbol() *ast.Symbol {
	return s.Macros().LuaTupleNominalSymbol()
}

// ---------------------------------------------------------------------------
// Module ids (upstream lines 339-354)
// ---------------------------------------------------------------------------

// SetModuleIDBySymbol records the luau id holding moduleSymbol's exports
// table (upstream setModuleIdBySymbol; transformSourceFile maps the file's
// module symbol to `exports`).
func (s *State) SetModuleIDBySymbol(moduleSymbol *ast.Symbol, moduleID luau.AnyIdentifier) {
	s.moduleIDBySymbol[moduleSymbol] = moduleID
}

// GetModuleIDFromSymbol returns the recorded module id, panicking if absent
// (upstream getModuleIdFromSymbol assert).
func (s *State) GetModuleIDFromSymbol(moduleSymbol *ast.Symbol) luau.AnyIdentifier {
	id, ok := s.moduleIDBySymbol[moduleSymbol]
	if !ok {
		panic("transformer: GetModuleIDFromSymbol: no module id recorded for symbol")
	}
	return id
}

// getModuleSymbolFromNode ports TransformState.getModuleSymbolFromNode: the
// export symbol of the nearest SourceFile or ModuleDeclaration ancestor
// (traversal.ts getModuleAncestor).
func (s *State) getModuleSymbolFromNode(node *ast.Node) *ast.Symbol {
	moduleAncestor := ast.FindAncestor(node, func(n *ast.Node) bool {
		return ast.IsSourceFile(n) || ast.IsModuleDeclaration(n)
	})
	location := moduleAncestor
	if !ast.IsSourceFile(moduleAncestor) {
		location = moduleAncestor.Name()
	}
	exportSymbol := s.Checker.GetSymbolAtLocation(location)
	if exportSymbol == nil {
		panic("transformer: getModuleSymbolFromNode: no module symbol") // upstream assert
	}
	return exportSymbol
}

// GetModuleIDPropertyAccess ports TransformState.getModuleIdPropertyAccess:
// `exports.<name>` (or the namespace container's id) when idSymbol is exported
// from its module, else nil. This is how `export let x` reads/writes become
// `exports.x` (transformIdentifier.ts:161-171, transformVariableStatement.ts:
// 26-40).
func (s *State) GetModuleIDPropertyAccess(idSymbol *ast.Symbol) *luau.PropertyAccessExpression {
	if idSymbol.ValueDeclaration != nil {
		moduleSymbol := s.getModuleSymbolFromNode(idSymbol.ValueDeclaration)
		if alias, ok := s.GetModuleExportsAliasMap(moduleSymbol)[idSymbol]; ok {
			return luau.NewPropertyAccess(s.GetModuleIDFromSymbol(moduleSymbol), alias)
		}
	}
	return nil
}
