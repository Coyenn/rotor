package transformer

import (
	"rotor/internal/luau"
	"rotor/tsgo/ast"
)

// This file implements the rotor $git and $buildTime compile-time stamping
// macros — SUPERSET extensions with no rbxtsc counterpart. Surface:
//
//	$git("sha")    -> "a1b2c3d"   (short 7-char commit hash, or "")
//	$git("branch") -> "main"      (current branch, or "" in detached HEAD)
//	$git("tag")    -> "v1.2.0"    (nearest tag pointing at HEAD, or "")
//	$git("dirty")  -> true        (working tree has uncommitted changes)
//	$buildTime()   -> "2026-06-13T09:41:05Z"   (ISO-8601 of build time)
//
// DETERMINISM:
//   - $git is STABLE within a commit: the same checkout produces the same sha/
//     branch/tag/dirty, so a $git-using file rebuilds to identical bytes from
//     the same working tree. It is NOT hostile to incremental caching.
//   - $buildTime is INTENTIONALLY non-deterministic — every build stamps the
//     current wall-clock time, so a file that uses $buildTime always changes and
//     SHOULD bust incremental caching for that file. Use it sparingly.
//
// The build/VCS data is read through a StampProvider (state.go) so the values
// come from one seam: the real implementation reads `.git` natively and
// time.Now(); tests inject a fake for stable goldens.
//
// INTERCEPTION POINT (why here): same rationale as $env/$asset (see
// envmacro.go). The `$git`/`$buildTime` identifiers have no runtime value, so
// the leading call link is consumed before TransformExpression sees the base
// identifier. Dispatch keys on the IDENTIFIER's symbol (the globals injected by
// compile's synthetic declaration file), exact and shadowing-safe. A bare
// `$git`/`$buildTime` outside a call is rejected in TransformIdentifier
// (identifier.go).

// StampProvider supplies the build/VCS values for the $git and $buildTime
// macros. It is the single seam through which the macros read the outside world,
// so tests can inject a stable fake (deterministic goldens) while the real
// implementation (internal/stampprovider) reads `.git` natively and time.Now().
//
// GitSHA/GitBranch/GitTag return "" when the field is unavailable (not a git
// repo, detached HEAD, no tag); GitDirty returns false when git/.git is absent.
// BuildTime returns the build's ISO-8601 timestamp.
type StampProvider interface {
	GitSHA() string    // short 7-char commit hash, or ""
	GitBranch() string // current branch name, or "" (detached HEAD)
	GitTag() string    // nearest tag pointing at HEAD, or ""
	GitDirty() bool    // working tree has uncommitted changes
	BuildTime() string // ISO-8601 timestamp of the build
}

// noopStampProvider is the fallback when no provider is attached: every git
// field is empty/false and BuildTime is empty. Keeps checker-light states from
// panicking on $git/$buildTime.
type noopStampProvider struct{}

func (noopStampProvider) GitSHA() string    { return "" }
func (noopStampProvider) GitBranch() string { return "" }
func (noopStampProvider) GitTag() string    { return "" }
func (noopStampProvider) GitDirty() bool    { return false }
func (noopStampProvider) BuildTime() string { return "" }

// gitMacroSymbol / buildTimeMacroSymbol resolve the global symbols (memoized by
// the MacroManager's lazy SYMBOL registry). nil when the program has no such
// declaration (checker-light test projects).
func gitMacroSymbol(s *State) *ast.Symbol {
	if s.Checker == nil {
		return nil
	}
	return s.Macros().Symbol("$git")
}

func buildTimeMacroSymbol(s *State) *ast.Symbol {
	if s.Checker == nil {
		return nil
	}
	return s.Macros().Symbol("$buildTime")
}

func isGitMacroNode(s *State, node *ast.Node) bool {
	node = SkipDownwards(node)
	if !ast.IsIdentifier(node) || node.Text() != "$git" {
		return false
	}
	symbol := gitMacroSymbol(s)
	return symbol != nil && s.Checker.GetSymbolAtLocation(node) == symbol
}

func isBuildTimeMacroNode(s *State, node *ast.Node) bool {
	node = SkipDownwards(node)
	if !ast.IsIdentifier(node) || node.Text() != "$buildTime" {
		return false
	}
	symbol := buildTimeMacroSymbol(s)
	return symbol != nil && s.Checker.GetSymbolAtLocation(node) == symbol
}

// gitStringLiteralText unwraps a string-literal-like argument, returning its
// cooked text. ok=false for any other expression.
func gitStringLiteralText(node *ast.Node) (string, bool) {
	node = SkipDownwards(node)
	if ast.IsStringLiteralLike(node) {
		return node.Text(), true
	}
	return "", false
}

// stamps returns the pass StampProvider, falling back to a no-op provider when
// none is attached (checker-light states) so the macros emit empty/false rather
// than panicking.
func stamps(s *State) StampProvider {
	if s.Stamps != nil {
		return s.Stamps
	}
	return noopStampProvider{}
}

// gitFieldExpression maps a $git field name to its inlined Luau literal.
func gitFieldExpression(s *State, node *ast.Node, field string) luau.Expression {
	provider := stamps(s)
	switch field {
	case "sha":
		return luau.Str(provider.GitSHA())
	case "branch":
		return luau.Str(provider.GitBranch())
	case "tag":
		return luau.Str(provider.GitTag())
	case "dirty":
		return luau.Bool(provider.GitDirty())
	default:
		// Unreachable after typecheck (the ambient overloads restrict field to
		// the four string-literal unions); defensive for checker-light paths.
		s.Diags.Add(DiagRotorGitBadField(node, field))
		return luau.NewNone()
	}
}

// interceptGitChain is the transformOptionalChain hook for `$git(field)`.
func interceptGitChain(s *State, chain []chainItem, expression *ast.Node) (luau.Expression, bool) {
	if len(chain) == 0 || !isGitMacroNode(s, expression) {
		return nil, false
	}

	item := chain[0]
	if item.kind != chainCall {
		s.Diags.Add(DiagRotorGitBadUsage(item.node))
		return luau.NewNone(), true
	}
	if len(item.args) != 1 {
		s.Diags.Add(DiagRotorGitBadUsage(item.node))
		return luau.NewNone(), true
	}
	field, ok := gitStringLiteralText(item.args[0])
	if !ok {
		s.Diags.Add(DiagRotorGitNonLiteralArg(item.args[0]))
		return luau.NewNone(), true
	}

	value := gitFieldExpression(s, item.node, field)
	return transformOptionalChainInner(s, chain, value, nil, 1), true
}

// interceptBuildTimeChain is the transformOptionalChain hook for `$buildTime()`.
func interceptBuildTimeChain(s *State, chain []chainItem, expression *ast.Node) (luau.Expression, bool) {
	if len(chain) == 0 || !isBuildTimeMacroNode(s, expression) {
		return nil, false
	}

	item := chain[0]
	if item.kind != chainCall {
		s.Diags.Add(DiagRotorBuildTimeBadUsage(item.node))
		return luau.NewNone(), true
	}
	if len(item.args) != 0 {
		// Unreachable after typecheck (the ambient signature takes no args);
		// defensive for checker-light paths.
		s.Diags.Add(DiagRotorBuildTimeBadUsage(item.node))
		return luau.NewNone(), true
	}

	value := luau.Str(stamps(s).BuildTime())
	return transformOptionalChainInner(s, chain, value, nil, 1), true
}
