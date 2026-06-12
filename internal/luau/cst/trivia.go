package cst

import (
	"strings"

	"rotor/internal/luau/lex"
)

// Trivia is a single whitespace or comment token (non-significant source text).
type Trivia struct {
	Kind  lex.Kind // lex.Whitespace or lex.Comment
	Text  string
	Start lex.Pos
}

// TokenRef wraps a significant token with its attached leading and trailing trivia.
// Leading+Token.Text+Trailing, concatenated across all refs in order, reproduces the
// source exactly.
type TokenRef struct {
	Leading  []Trivia
	Token    lex.Token
	Trailing []Trivia
}

func (r TokenRef) text() string {
	var b strings.Builder
	for _, tr := range r.Leading {
		b.WriteString(tr.Text)
	}
	b.WriteString(r.Token.Text)
	for _, tr := range r.Trailing {
		b.WriteString(tr.Text)
	}
	return b.String()
}

func isTrivia(k lex.Kind) bool { return k == lex.Whitespace || k == lex.Comment }

func triviaOf(t lex.Token) Trivia { return Trivia{Kind: t.Kind, Text: t.Text, Start: t.Start} }

func hasNewline(s string) bool { return strings.IndexByte(s, '\n') >= 0 }

// AttachTrivia converts a source string into a sequence of TokenRefs (terminated by
// an EOF ref). Trailing trivia of a token runs up to — but not including — the first
// following whitespace run that contains a newline; that run and everything after it
// become the leading trivia of the next significant token.
func AttachTrivia(src string) []TokenRef {
	toks, _ := lex.Tokenize(src)
	var refs []TokenRef
	var leading []Trivia
	i := 0
	for i < len(toks) {
		for i < len(toks) && isTrivia(toks[i].Kind) {
			leading = append(leading, triviaOf(toks[i]))
			i++
		}
		sig := toks[i]
		i++
		ref := TokenRef{Leading: leading, Token: sig}
		leading = nil
		for i < len(toks) && isTrivia(toks[i].Kind) {
			if toks[i].Kind == lex.Whitespace && hasNewline(toks[i].Text) {
				break
			}
			ref.Trailing = append(ref.Trailing, triviaOf(toks[i]))
			i++
		}
		refs = append(refs, ref)
		if sig.Kind == lex.EOF {
			break
		}
	}
	return refs
}

// Flatten concatenates every TokenRef's full text. For refs produced by AttachTrivia
// this equals the original source.
func Flatten(refs []TokenRef) string {
	var b strings.Builder
	for _, r := range refs {
		for _, tr := range r.Leading {
			b.WriteString(tr.Text)
		}
		b.WriteString(r.Token.Text)
		for _, tr := range r.Trailing {
			b.WriteString(tr.Text)
		}
	}
	return b.String()
}
