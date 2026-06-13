package transformer

import (
	"errors"

	"rotor/internal/luau"
	"rotor/tsgo/ast"
)

// This file implements the rotor $asset compile-time asset macro — a SUPERSET
// extension with no rbxtsc counterpart (the headline rotor 2.0 feature).
// Surface:
//
//	$asset("assets/logo.png") -> "rbxassetid://123"
//
// The single string-literal argument is a path resolved relative to the
// project root (or relative to the importing file when it begins with `./`
// or `../`). The macro resolves the file's content hash to a Roblox asset id
// through State.Assets (an AssetResolver) — a lockfile cache hit is fully
// offline and deterministic; a cache miss with ROBLOX_API_KEY set uploads
// via Open Cloud — and inlines the result as a single Luau string literal.
//
// INTERCEPTION POINT (why here): same rationale as $env (see envmacro.go).
// The `$asset` identifier has no runtime value, so it must be consumed before
// the regular identifier transform emits the invalid Luau identifier
// `$asset`. transformOptionalChain flattens the chain FIRST, then transforms
// the base, so a single hook there (optionalchain.go) sees `$asset(...)`
// before the base identifier is transformed. Unlike $env, $asset has ONLY the
// call form — there are no `$asset.X` / `$asset["X"]` accesses — so the hook
// only handles a leading chainCall. Dispatch keys on the IDENTIFIER's symbol
// (the global `$asset` const injected by compile's synthetic declaration
// file), which is exact and shadowing-safe: a user-defined local `$asset`
// resolves to a different symbol and compiles as plain code. A bare `$asset`
// that is NOT the head of a call is rejected in TransformIdentifier
// (identifier.go).

// AssetResolver resolves a project-relative (or importing-file-relative) asset
// path to its Roblox asset id (the numeric id only, without the
// `rbxassetid://` prefix). Implemented in internal/assetresolve and plumbed
// onto State.Assets per compile pass; nil in transformer-level unit tests.
//
// Resolve returns one of the sentinel errors below (mapped to specific
// diagnostics) or any other error (mapped to a generic resolve-failed
// diagnostic). importerPath is the absolute path of the source file
// containing the $asset call, used to resolve `./`-relative paths.
type AssetResolver interface {
	Resolve(importerPath, path string) (id string, err error)
}

// Sentinel errors the transformer maps to specific diagnostics. Resolvers
// return these (wrapped is fine — the transformer uses errors.Is).
var (
	// ErrAssetFileNotFound: the resolved path names no file on disk.
	ErrAssetFileNotFound = errors.New("asset file not found")
	// ErrAssetNotCached: the file is not in the lockfile and could not be
	// uploaded (no cloud client / API key).
	ErrAssetNotCached = errors.New("asset not cached")
)

// assetMacroSymbol resolves the global `$asset` symbol (memoized by the
// MacroManager's lazy SYMBOL registry). nil when the program has no $asset
// declaration (checker-light test projects that bypass compile's synthetic
// declaration injection).
func assetMacroSymbol(s *State) *ast.Symbol {
	if s.Checker == nil {
		return nil
	}
	return s.Macros().Symbol("$asset")
}

// isAssetMacroNode reports whether node (skipped downwards) is an identifier
// bound to THE global $asset symbol. The text pre-filter keeps the hot
// transform paths free of symbol lookups.
func isAssetMacroNode(s *State, node *ast.Node) bool {
	node = SkipDownwards(node)
	if !ast.IsIdentifier(node) || node.Text() != "$asset" {
		return false
	}
	symbol := assetMacroSymbol(s)
	return symbol != nil && s.Checker.GetSymbolAtLocation(node) == symbol
}

// assetStringLiteralText unwraps a string-literal-like argument ("..." or
// `...` without substitutions), returning its cooked text. ok=false for any
// other expression — $asset resolves at compile time, so a dynamic path
// cannot be inlined.
func assetStringLiteralText(node *ast.Node) (string, bool) {
	node = SkipDownwards(node)
	if ast.IsStringLiteralLike(node) {
		return node.Text(), true
	}
	return "", false
}

// resolveAssetExpression resolves path through the pass asset resolver into
// the inlined Luau string literal `"rbxassetid://<id>"`, or adds a diagnostic
// and returns None. node is the call expression (diagnostic position source).
func resolveAssetExpression(s *State, node *ast.Node, path string) luau.Expression {
	if s.Assets == nil {
		// No resolver attached (unit tests / single-file path without project
		// context). A clear diagnostic, never a panic.
		s.Diags.Add(DiagRotorAssetNoResolver(node))
		return luau.NewNone()
	}
	importer := ""
	if s.SourceFile != nil {
		importer = s.SourceFile.FileName()
	}
	id, err := s.Assets.Resolve(importer, path)
	if err != nil {
		switch {
		case errors.Is(err, ErrAssetFileNotFound):
			s.Diags.Add(DiagRotorAssetFileNotFound(node, path))
		case errors.Is(err, ErrAssetNotCached):
			s.Diags.Add(DiagRotorAssetNotCached(node, path))
		default:
			s.Diags.Add(DiagRotorAssetResolveFailed(node, path, err.Error()))
		}
		return luau.NewNone()
	}
	return luau.Str("rbxassetid://" + id)
}

// interceptAssetChain is the transformOptionalChain hook: when the chain's
// base expression is the $asset global, the leading call link is folded to its
// resolved asset id and the rest of the chain (if any) continues from that
// literal. handled=false means "not $asset — proceed normally". An optional
// call marker (`$asset?.("x")`) is accepted and ignored: $asset is never nil.
func interceptAssetChain(s *State, chain []chainItem, expression *ast.Node) (luau.Expression, bool) {
	if len(chain) == 0 || !isAssetMacroNode(s, expression) {
		return nil, false
	}

	item := chain[0]
	if item.kind != chainCall {
		// Bare property/element access off $asset ($asset.X / $asset["X"]) —
		// $asset has no such surface. The checker already rejected these (the
		// ambient type is a function with no members); defensive for
		// checker-light paths.
		s.Diags.Add(DiagRotorAssetBadUsage(item.node))
		return luau.NewNone(), true
	}

	// $asset("path")
	if len(item.args) != 1 {
		// Unreachable after typecheck (the ambient signature takes exactly one
		// arg); defensive for checker-light paths.
		s.Diags.Add(DiagRotorAssetBadUsage(item.node))
		return luau.NewNone(), true
	}
	path, ok := assetStringLiteralText(item.args[0])
	if !ok {
		s.Diags.Add(DiagRotorAssetNonLiteralArg(item.args[0]))
		return luau.NewNone(), true
	}

	value := resolveAssetExpression(s, item.node, path)

	// Continue any remaining chain links off the inlined literal, e.g.
	// `$asset("a.png").size()`.
	return transformOptionalChainInner(s, chain, value, nil, 1), true
}
