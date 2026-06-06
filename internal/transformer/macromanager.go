package transformer

import (
	"strings"

	"rotor/internal/luau"
	"rotor/tsgo/ast"
	"rotor/tsgo/checker"
)

// This file ports TSTransformer/classes/MacroManager.ts and
// TSTransformer/macros/types.ts: ONE component owning all macro
// identification, shared per compilation pass via MultiState (upstream
// constructs the MacroManager once per Program and threads it through
// TransformServices).
//
// Phase 3a state of the tables:
//   - CONSTRUCTOR_MACROS are fully implemented (constructormacros.go).
//   - IDENTIFIER_MACROS / CALL_MACROS entries are registered with nil
//     implementations: a nil Macro on a returned entry means "known upstream
//     macro, not implemented in rotor yet" — call sites raise
//     rotorNotYetSupported with the macro name instead of emitting
//     silently-wrong output.
//   - PROPERTY_CALL_MACROS method tables (Array.push, Map.set, ...) land in
//     Phase 3b; until then the compiler-types fallbacks below cover detection.
//
// Fallback semantics (the Phase 2 stand-ins, centralized): every macro
// upstream registers is declared by @rbxts/compiler-types, so a
// compiler-types-declared symbol at a macro hook is treated as a known macro
// even when it is missing from the tables. Upstream instead throws
// ProjectError at construction when a registration name cannot be resolved
// ("You may need to update your @rbxts/compiler-types!"); rotor skips the
// entry so checker-light test projects keep working (same divergence as the
// AmbientSymbol nil-return pattern).

// ---------------------------------------------------------------------------
// Macro signatures — macros/types.ts
// ---------------------------------------------------------------------------

// IdentifierMacro ports macros/types.ts IdentifierMacro.
type IdentifierMacro func(s *State, node *ast.Node) luau.Expression

// ConstructorMacro ports macros/types.ts ConstructorMacro (node is the
// ts.NewExpression).
type ConstructorMacro func(s *State, node *ast.Node) luau.Expression

// CallMacro ports macros/types.ts CallMacro.
type CallMacro func(s *State, node *ast.Node, expression luau.Expression, args []luau.Expression) luau.Expression

// PropertyCallMacro ports macros/types.ts PropertyCallMacro.
type PropertyCallMacro func(s *State, node *ast.Node, expression luau.Expression, args []luau.Expression) luau.Expression

// IdentifierMacroEntry / CallMacroEntry / PropertyCallMacroEntry pair a macro
// with its upstream registration name (used in diagnostics). Macro == nil
// means the macro exists upstream but rotor has not implemented it yet.
type IdentifierMacroEntry struct {
	Name  string
	Macro IdentifierMacro
}

type CallMacroEntry struct {
	Name  string
	Macro CallMacro
}

type PropertyCallMacroEntry struct {
	Name  string
	Macro PropertyCallMacro
}

// ---------------------------------------------------------------------------
// Registration tables
// ---------------------------------------------------------------------------

// identifierMacroTable ports macros/identifierMacros.ts IDENTIFIER_MACROS.
// Phase 3b: Promise => state.TS(node, "Promise").
var identifierMacroTable = map[string]IdentifierMacro{
	"Promise": nil,
}

// callMacroTable ports macros/callMacros.ts CALL_MACROS (names only; the
// implementations land in Phase 3b).
var callMacroTable = map[string]CallMacro{
	"assert":         nil,
	"typeOf":         nil,
	"typeIs":         nil,
	"classIs":        nil,
	"identity":       nil,
	"$range":         nil,
	"$tuple":         nil,
	"$getModuleTree": nil,
}

// constructorMacroTable ports macros/constructorMacros.ts CONSTRUCTOR_MACROS
// (all implemented — constructormacros.go). Populated in init() because the
// macro funcs transitively reference State helpers that reach back to the
// MacroManager (Go initialization-cycle rule).
var constructorMacroTable map[string]ConstructorMacro

func init() {
	constructorMacroTable = map[string]ConstructorMacro{
		"ArrayConstructor": arrayConstructorMacro,
		"SetConstructor":   setConstructorMacro,
		"MapConstructor":   mapConstructorMacro,
		"WeakSetConstructor": func(s *State, node *ast.Node) luau.Expression {
			return wrapWeak(s, node, setConstructorMacro)
		},
		"WeakMapConstructor": func(s *State, node *ast.Node) luau.Expression {
			return wrapWeak(s, node, mapConstructorMacro)
		},
		"ReadonlyMapConstructor": mapConstructorMacro,
		"ReadonlySetConstructor": setConstructorMacro,
	}
}

// NominalLuaTupleName ports Shared/constants.ts NOMINAL_LUA_TUPLE_NAME.
const NominalLuaTupleName = "_nominal_LuaTuple"

// ---------------------------------------------------------------------------
// MacroManager
// ---------------------------------------------------------------------------

// MacroManager ports classes/MacroManager.ts: symbol -> macro lookup maps
// built once per compilation pass from the checker's ambient declarations,
// plus the SYMBOL_NAMES ambient-symbol registry (rotor resolves those lazily;
// see Symbol).
type MacroManager struct {
	chk *checker.Checker // nil in checker-free mechanics tests

	// symbols ports the SYMBOL_NAMES map, lazily resolved and memoized
	// (including misses — presence in the map marks "resolved").
	symbols map[string]*ast.Symbol

	identifierMacros   map[*ast.Symbol]*IdentifierMacroEntry
	callMacros         map[*ast.Symbol]*CallMacroEntry
	constructorMacros  map[*ast.Symbol]ConstructorMacro
	propertyCallMacros map[*ast.Symbol]*PropertyCallMacroEntry

	// luaTupleNominal caches the `_nominal_LuaTuple` property symbol resolved
	// from the ambient LuaTuple<T> alias (see LuaTupleNominalSymbol).
	luaTupleNominal         *ast.Symbol
	luaTupleNominalResolved bool
}

// NewMacroManager builds the symbol->macro maps, mirroring the upstream
// constructor: each registration name is resolved through
// typeChecker.resolveName against the ambient declarations; constructor
// macros are keyed by the construct-signature symbol of the GLOBAL interface
// (so user-defined shadowing types won't collide). Unresolvable names are
// skipped (see the header note on the upstream-throw divergence).
func NewMacroManager(chk *checker.Checker) *MacroManager {
	m := &MacroManager{
		chk:                chk,
		symbols:            make(map[string]*ast.Symbol),
		identifierMacros:   make(map[*ast.Symbol]*IdentifierMacroEntry),
		callMacros:         make(map[*ast.Symbol]*CallMacroEntry),
		constructorMacros:  make(map[*ast.Symbol]ConstructorMacro),
		propertyCallMacros: make(map[*ast.Symbol]*PropertyCallMacroEntry),
	}
	if chk == nil {
		return m
	}

	for name, macro := range identifierMacroTable {
		if symbol := chk.ResolveName(name, nil, ast.SymbolFlagsVariable, false); symbol != nil {
			m.identifierMacros[symbol] = &IdentifierMacroEntry{Name: name, Macro: macro}
		}
	}

	for name, macro := range callMacroTable {
		if symbol := chk.ResolveName(name, nil, ast.SymbolFlagsFunction, false); symbol != nil {
			m.callMacros[symbol] = &CallMacroEntry{Name: name, Macro: macro}
		}
	}

	for name, macro := range constructorMacroTable {
		symbol := chk.ResolveName(name, nil, ast.SymbolFlagsInterface, false)
		if symbol == nil {
			continue
		}
		// getFirstDeclarationOrThrow(symbol, ts.isInterfaceDeclaration) +
		// getConstructorSymbol(interfaceDec): FIRST interface declaration only.
		for _, declaration := range symbol.Declarations {
			if !ast.IsInterfaceDeclaration(declaration) {
				continue
			}
			if constructSymbol := interfaceConstructSymbol(declaration); constructSymbol != nil {
				m.constructorMacros[constructSymbol] = macro
			}
			break
		}
	}

	// PROPERTY_CALL_MACROS registration (className -> method symbol maps)
	// lands with the Phase 3b macro tables; m.propertyCallMacros stays empty
	// and GetPropertyCallMacro's compiler-types fallback covers detection.

	return m
}

// interfaceConstructSymbol ports MacroManager.ts getConstructorSymbol: the
// symbol of the first construct-signature member of the interface
// declaration, or nil (upstream throws ProjectError).
func interfaceConstructSymbol(interfaceDec *ast.Node) *ast.Symbol {
	for _, member := range interfaceDec.AsInterfaceDeclaration().Members.Nodes {
		if ast.IsConstructSignatureDeclaration(member) {
			return member.Symbol()
		}
	}
	return nil
}

// Symbol ports MacroManager.getSymbolOrThrow + the SYMBOL_NAMES registration:
// `typeChecker.resolveName(symbolName, undefined, ts.SymbolFlags.All, false)`,
// memoized (including misses). Returns nil instead of throwing for projects
// without @rbxts/compiler-types (callers nil-guard).
func (m *MacroManager) Symbol(name string) *ast.Symbol {
	if symbol, ok := m.symbols[name]; ok {
		return symbol
	}
	var symbol *ast.Symbol
	if m.chk != nil {
		symbol = m.chk.ResolveName(name, nil, ast.SymbolFlagsAll, false)
	}
	m.symbols[name] = symbol
	return symbol
}

// LuaTupleNominalSymbol resolves the `_nominal_LuaTuple` property symbol from
// the ambient LuaTuple<T> type alias, memoized. Ports the MacroManager
// constructor tail: find the LuaTuple TypeAliasDeclaration, take the type at
// that location, and grab its NOMINAL_LUA_TUPLE_NAME property. nil when the
// project has no @rbxts/compiler-types.
func (m *MacroManager) LuaTupleNominalSymbol() *ast.Symbol {
	if m.luaTupleNominalResolved {
		return m.luaTupleNominal
	}
	m.luaTupleNominalResolved = true
	if luaTupleSymbol := m.Symbol("LuaTuple"); luaTupleSymbol != nil {
		for _, declaration := range luaTupleSymbol.Declarations {
			if ast.IsTypeAliasDeclaration(declaration) {
				t := m.chk.GetTypeAtLocation(declaration)
				m.luaTupleNominal = m.chk.GetPropertyOfType(t, NominalLuaTupleName)
				break
			}
		}
	}
	return m.luaTupleNominal
}

// GetIdentifierMacro ports macroManager.getIdentifierMacro. The fallback is
// DELIBERATELY broader than upstream's table: rotor's Phase 2 stand-in treats
// ANY compiler-types-declared symbol in identifier position as a known macro
// (rejected loudly by the caller), because upstream's surrounding guards
// (noConstructorMacroWithoutNew, noMacroExtends, noIndexWithoutCall at
// transformIdentifier.ts L137-159) are not all ported yet. Phase 3b narrows
// this to the real table + those guards.
func (m *MacroManager) GetIdentifierMacro(symbol *ast.Symbol) *IdentifierMacroEntry {
	if entry, ok := m.identifierMacros[symbol]; ok {
		return entry
	}
	if isCompilerTypesSymbol(symbol) {
		return &IdentifierMacroEntry{Name: symbol.Name}
	}
	return nil
}

// GetCallMacro ports macroManager.getCallMacro. Fallback: every upstream
// CALL_MACROS entry (typeOf, typeIs, identity, $range, ...) is a
// `declare function` in @rbxts/compiler-types; restricting to
// FunctionDeclaration declarations keeps compiler-types TYPES (e.g. a
// Callback-typed value's anonymous function type) callable.
func (m *MacroManager) GetCallMacro(symbol *ast.Symbol) *CallMacroEntry {
	if entry, ok := m.callMacros[symbol]; ok {
		return entry
	}
	if isCompilerTypesSymbol(symbol) && symbolHasDeclarationOfKind(symbol, ast.IsFunctionDeclaration) {
		return &CallMacroEntry{Name: macroDisplayName(symbol)}
	}
	return nil
}

// IsTypeCheckCallMacro stands in for `macro === CALL_MACROS.typeIs ||
// macro === CALL_MACROS.typeOf` (isValidMethodIndexWithoutCall.ts L24-29).
func (m *MacroManager) IsTypeCheckCallMacro(symbol *ast.Symbol) bool {
	entry := m.GetCallMacro(symbol)
	return entry != nil && (entry.Name == "typeIs" || entry.Name == "typeOf")
}

// GetConstructorMacro ports macroManager.getConstructorMacro: table-only, no
// fallback — non-macro construct signatures take transformNewExpression's
// `X.new(args)` fallback, exactly as upstream.
func (m *MacroManager) GetConstructorMacro(symbol *ast.Symbol) ConstructorMacro {
	return m.constructorMacros[symbol]
}

// GetPropertyCallMacro ports macroManager.getPropertyCallMacro. Fallback:
// upstream PROPERTY_CALL_MACROS keys are method-member symbols of
// compiler-types interfaces (Array.push, Map.set, String.size, ...).
// Interface symbols themselves (e.g. the Array type of `arr` in `arr[0]`) are
// NOT macros. Upstream's macro-only-class assert ("Macro X.y() is not
// implemented!") is subsumed by the sentinel until the 3b tables land.
func (m *MacroManager) GetPropertyCallMacro(symbol *ast.Symbol) *PropertyCallMacroEntry {
	if entry, ok := m.propertyCallMacros[symbol]; ok {
		return entry
	}
	if isCompilerTypesSymbol(symbol) && symbolHasDeclarationOfKind(symbol, isMethodLikeDeclaration) {
		return &PropertyCallMacroEntry{Name: macroDisplayName(symbol)}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Fallback helpers (the centralized Phase 2 stand-ins)
// ---------------------------------------------------------------------------

// isCompilerTypesSymbol reports whether symbol is declared by the
// @rbxts/compiler-types package — upstream's MacroManager builds its macro
// tables exclusively from those declaration files, so this is the stand-in
// for "symbol has a macro" wherever the real tables are not populated yet.
func isCompilerTypesSymbol(symbol *ast.Symbol) bool {
	for _, declaration := range symbol.Declarations {
		if sf := ast.GetSourceFileOfNode(declaration); sf != nil &&
			strings.Contains(sf.FileName(), "node_modules/@rbxts/compiler-types/") {
			return true
		}
	}
	return false
}

func symbolHasDeclarationOfKind(symbol *ast.Symbol, check func(*ast.Node) bool) bool {
	for _, declaration := range symbol.Declarations {
		if check(declaration) {
			return true
		}
	}
	return false
}

func isMethodLikeDeclaration(node *ast.Node) bool {
	return ast.IsMethodSignatureDeclaration(node) || ast.IsMethodDeclaration(node)
}

// macroDisplayName renders a macro symbol for diagnostics, mirroring
// upstream's `${symbol.parent.name}.${symbol.name}` assert text where a
// parent exists (e.g. "Array.push").
func macroDisplayName(symbol *ast.Symbol) string {
	if symbol.Parent != nil && luau.IsValidIdentifier(symbol.Parent.Name) {
		return symbol.Parent.Name + "." + symbol.Name
	}
	return symbol.Name
}
