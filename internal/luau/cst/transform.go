package cst

import (
	"strings"

	"rotor/internal/luau/lex"
)

// EachNode calls visit for n and every descendant node, in source order (pre-order).
func EachNode(n Node, visit func(Node)) {
	visit(n)
	n.tokens(func(ref *TokenRef, child Node) {
		if child != nil {
			EachNode(child, visit)
		}
	})
}

// StringValue decodes a string-literal node to its textual value. It supports short
// strings ('...'/"...") with common escapes and long strings ([[...]]/[=[...]=]).
// ok is false for anything that is not a plain string literal.
func StringValue(s *String) (value string, ok bool) {
	raw := s.Tok.Token.Text
	if len(raw) >= 2 && (raw[0] == '"' || raw[0] == '\'') && raw[len(raw)-1] == raw[0] {
		return unescapeShort(raw[1 : len(raw)-1]), true
	}
	if body, ok := longStringBody(raw); ok {
		return body, true
	}
	return "", false
}

func unescapeShort(s string) string {
	if !strings.ContainsRune(s, '\\') {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			switch s[i] {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'r':
				b.WriteByte('\r')
			default:
				b.WriteByte(s[i]) // \\, \", \', \/ and others: keep the escaped char
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// longStringBody returns the inner text of a long string [[..]] / [=[..]=].
func longStringBody(raw string) (string, bool) {
	if len(raw) < 2 || raw[0] != '[' {
		return "", false
	}
	i := 1
	for i < len(raw) && raw[i] == '=' {
		i++
	}
	if i >= len(raw) || raw[i] != '[' {
		return "", false
	}
	level := i - 1
	open := level + 2
	close := level + 2
	if len(raw) < open+close {
		return "", false
	}
	return raw[open : len(raw)-close], true
}

// lastRef returns the last non-EOF TokenRef beneath n, or nil if there is none.
func lastRef(n Node) *TokenRef {
	var found *TokenRef
	EachToken(n, func(ref *TokenRef) {
		if ref.Token.Kind != lex.EOF {
			found = ref
		}
	})
	return found
}

// UnparseWith is Unparse with substitutions: any node present in replace is emitted
// as its replacement string instead of its own tokens, while the node's surrounding
// trivia (the leading trivia of its first token and the trailing trivia of its last
// token) is preserved so the substitution slots cleanly into the formatted output.
// The tree itself is never mutated — this is how the bundler rewrites require(...)
// calls. With an empty map it equals Unparse.
func UnparseWith(n Node, replace map[Node]string) string {
	var b strings.Builder
	var walk func(Node)
	walk = func(n Node) {
		if rep, ok := replace[n]; ok {
			if fr := firstRef(n); fr != nil {
				for _, tr := range fr.Leading {
					b.WriteString(tr.Text)
				}
			}
			b.WriteString(rep)
			if lr := lastRef(n); lr != nil {
				for _, tr := range lr.Trailing {
					b.WriteString(tr.Text)
				}
			}
			return
		}
		n.tokens(func(ref *TokenRef, child Node) {
			switch {
			case ref != nil:
				b.WriteString(ref.text())
			case child != nil:
				walk(child)
			}
		})
	}
	walk(n)
	return b.String()
}
