package cst

import (
	"strings"

	"rotor/internal/luau/lex"
)

// Minify parses src and returns minified Luau. It preserves leading `--!` directive
// comments (e.g. `--!strict`, `--!native`), which affect how Luau compiles the file,
// but drops every other comment and all superfluous whitespace. Diagnostics from
// parsing are returned; on a clean parse the result is semantically identical to src
// (no variable renaming — that is a later minifier pass).
func Minify(src string) (string, []Diagnostic) {
	file, diags := Parse(src)
	var b strings.Builder
	for _, d := range directives(file) {
		b.WriteString(d)
		b.WriteByte('\n')
	}
	b.WriteString(Dense(file))
	return b.String(), diags
}

// directives returns the leading `--!` directive comment lines of the file, in order.
func directives(file *File) []string {
	ref := firstRef(file.Body)
	if ref == nil {
		ref = &file.EOF
	}
	var out []string
	for _, tr := range ref.Leading {
		if tr.Kind == lex.Comment && strings.HasPrefix(tr.Text, "--!") {
			out = append(out, tr.Text)
		}
	}
	return out
}

// firstRef returns the first non-EOF TokenRef beneath n, or nil if there is none.
func firstRef(n Node) *TokenRef {
	var found *TokenRef
	var walk func(Node)
	walk = func(n Node) {
		n.tokens(func(ref *TokenRef, child Node) {
			if found != nil {
				return
			}
			switch {
			case ref != nil:
				if ref.Token.Kind != lex.EOF {
					found = ref
				}
			case child != nil:
				walk(child)
			}
		})
	}
	walk(n)
	return found
}

// Dense serializes n to minified Luau: every significant token, no comments or
// superfluous whitespace, with a single space inserted only where two adjacent
// tokens would otherwise merge into a different token, and a ';' inserted between two
// statements when the next one begins with '(' (which would otherwise be misparsed as
// a call on the previous statement — the classic Lua ambiguity).
//
// Dense drops ALL trivia, including comments. Semantic `--!` directive comments must
// therefore be preserved by the caller (the minifier), not by Dense.
func Dense(n Node) string {
	w := &denseWriter{}
	w.node(n)
	return w.b.String()
}

type denseWriter struct {
	b        strings.Builder
	prev     string
	prevKind lex.Kind
	started  bool
}

func (w *denseWriter) tok(t lex.Token) {
	if t.Kind == lex.EOF {
		return
	}
	// No separator is ever needed adjacent to an interpolation chunk: its `, {, }
	// delimiters already separate it from neighbouring tokens.
	if w.started && !isInterpKind(w.prevKind) && !isInterpKind(t.Kind) && needsSeparator(w.prev, t.Text) {
		w.b.WriteByte(' ')
	}
	w.b.WriteString(t.Text)
	w.prev = t.Text
	w.prevKind = t.Kind
	w.started = true
}

// forced writes a fixed separator token (e.g. ";") without a leading space.
func (w *denseWriter) forced(s string) {
	w.b.WriteString(s)
	w.prev = s
	w.prevKind = lex.Symbol
	w.started = true
}

func isInterpKind(k lex.Kind) bool {
	return k == lex.InterpSimple || k == lex.InterpStart || k == lex.InterpMid || k == lex.InterpEnd
}

func (w *denseWriter) node(n Node) {
	switch v := n.(type) {
	case *File:
		w.block(v.Body)
	case *Block:
		w.block(v)
	default:
		n.tokens(func(ref *TokenRef, child Node) {
			switch {
			case ref != nil:
				w.tok(ref.Token)
			case child != nil:
				w.node(child)
			}
		})
	}
}

func (w *denseWriter) block(b *Block) {
	for i := range b.Stmts {
		if i > 0 && startsWithParen(b.Stmts[i]) {
			w.forced(";")
		}
		w.node(b.Stmts[i])
	}
}

// startsWithParen reports whether a statement's first significant token is '(' (the
// only statement-start that can glue onto a preceding statement as a call).
func startsWithParen(n Node) bool {
	t, ok := firstToken(n)
	return ok && t.Kind == lex.Symbol && t.Text == "("
}

// firstToken returns the first non-EOF token beneath n in source order.
func firstToken(n Node) (lex.Token, bool) {
	var found lex.Token
	var ok bool
	var walk func(Node)
	walk = func(n Node) {
		n.tokens(func(ref *TokenRef, child Node) {
			if ok {
				return
			}
			switch {
			case ref != nil:
				if ref.Token.Kind != lex.EOF {
					found = ref.Token
					ok = true
				}
			case child != nil:
				walk(child)
			}
		})
	}
	walk(n)
	return found, ok
}

// needsSeparator reports whether the tokens with texts a then b must be separated by
// whitespace to keep them as two distinct tokens. It is exact: it re-lexes the
// concatenation and checks that it still yields exactly the two original tokens.
func needsSeparator(a, b string) bool {
	toks, _ := lex.Tokenize(a + b)
	var sig []lex.Token
	for _, t := range toks {
		if t.Kind == lex.Whitespace || t.Kind == lex.Comment || t.Kind == lex.EOF {
			continue
		}
		sig = append(sig, t)
		if len(sig) > 2 {
			return true
		}
	}
	return !(len(sig) == 2 && sig[0].Text == a && sig[1].Text == b)
}
