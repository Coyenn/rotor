package luau

var luauReservedKeywords = map[string]struct{}{
	"and": {}, "break": {}, "do": {}, "else": {}, "elseif": {}, "end": {},
	"false": {}, "for": {}, "function": {}, "if": {}, "in": {}, "local": {},
	"nil": {}, "not": {}, "or": {}, "repeat": {}, "return": {}, "then": {},
	"true": {}, "until": {}, "while": {},
}

// IsReservedKeyword reports whether id is a Luau reserved keyword (the canonical
// set behind IsValidIdentifier). Exposed so presentation layers can mirror the
// set without duplicating the source of truth.
func IsReservedKeyword(id string) bool {
	_, ok := luauReservedKeywords[id]
	return ok
}

func isIdentifierStart(c byte) bool {
	return c == '_' || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
}

func isIdentifierPart(c byte) bool {
	return isIdentifierStart(c) || (c >= '0' && c <= '9')
}

// IsValidIdentifier reports whether id matches `^[A-Za-z_][A-Za-z0-9_]*$`
// (hand-rolled byte scan; any non-ASCII byte fails the classes, exactly like
// the regex it replaces) and is not a Luau reserved keyword.
func IsValidIdentifier(id string) bool {
	if id == "" || !isIdentifierStart(id[0]) {
		return false
	}
	for i := 1; i < len(id); i++ {
		if !isIdentifierPart(id[i]) {
			return false
		}
	}
	_, reserved := luauReservedKeywords[id]
	return !reserved
}

func isDecimalDigit(c byte) bool { return c >= '0' && c <= '9' }
func isBinaryDigit(c byte) bool  { return c == '0' || c == '1' }
func isHexDigit(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

// IsValidNumberLiteral reports whether text is a decimal, binary, or
// hexadecimal Luau number literal. It is a hand-rolled equivalent of the
// anchored regexes
//
//	decimal:     ^(?:\d[\d_]*(?:\.[\d_]*)?|\.\d[\d_]*)(?:[eE][+-]?_*\d[\d_]*)?$
//	binary:      ^0_*[bB]_*[01][01_]*$
//	hexadecimal: ^0_*[xX]_*[\da-fA-F][\da-fA-F_]*$
//
// (greedy scanning is safe: at every branch point the follow sets are
// disjoint, so no backtracking can change the outcome).
func IsValidNumberLiteral(text string) bool {
	if text == "" {
		return false
	}
	if text[0] == '0' {
		// 0_*[bB]... or 0_*[xX]...; otherwise fall through to decimal
		// (covers "0", "0_1", "0.5", "0e1", ...).
		i := 1
		for i < len(text) && text[i] == '_' {
			i++
		}
		if i < len(text) {
			switch text[i] {
			case 'b', 'B':
				return matchBaseDigits(text[i+1:], isBinaryDigit)
			case 'x', 'X':
				return matchBaseDigits(text[i+1:], isHexDigit)
			}
		}
	}
	return isDecimalLiteral(text)
}

// matchBaseDigits matches `^_*D[D_]*$` for the digit class D.
func matchBaseDigits(s string, isDigit func(byte) bool) bool {
	i := 0
	for i < len(s) && s[i] == '_' {
		i++
	}
	if i >= len(s) || !isDigit(s[i]) {
		return false
	}
	for i++; i < len(s); i++ {
		if s[i] != '_' && !isDigit(s[i]) {
			return false
		}
	}
	return true
}

// isDecimalLiteral matches the anchored decimal pattern documented on
// IsValidNumberLiteral.
func isDecimalLiteral(s string) bool {
	i, n := 0, len(s)
	switch {
	case isDecimalDigit(s[0]):
		// \d[\d_]*(?:\.[\d_]*)?
		for i++; i < n && (isDecimalDigit(s[i]) || s[i] == '_'); i++ {
		}
		if i < n && s[i] == '.' {
			for i++; i < n && (isDecimalDigit(s[i]) || s[i] == '_'); i++ {
			}
		}
	case s[0] == '.':
		// \.\d[\d_]*
		i++
		if i >= n || !isDecimalDigit(s[i]) {
			return false
		}
		for i++; i < n && (isDecimalDigit(s[i]) || s[i] == '_'); i++ {
		}
	default:
		return false
	}
	if i == n {
		return true
	}
	// (?:[eE][+-]?_*\d[\d_]*)$
	if s[i] != 'e' && s[i] != 'E' {
		return false
	}
	i++
	if i < n && (s[i] == '+' || s[i] == '-') {
		i++
	}
	for i < n && s[i] == '_' {
		i++
	}
	if i >= n || !isDecimalDigit(s[i]) {
		return false
	}
	for i++; i < n && (isDecimalDigit(s[i]) || s[i] == '_'); i++ {
	}
	return i == n
}

var luauMetamethods = map[string]struct{}{
	"__index": {}, "__newindex": {}, "__call": {}, "__concat": {}, "__unm": {},
	"__add": {}, "__sub": {}, "__mul": {}, "__div": {}, "__mod": {}, "__pow": {},
	"__tostring": {}, "__metatable": {}, "__eq": {}, "__lt": {}, "__le": {},
	"__mode": {}, "__gc": {}, "__len": {},
}

func IsMetamethod(id string) bool {
	_, ok := luauMetamethods[id]
	return ok
}

var luauReservedClassFields = map[string]struct{}{"__index": {}, "new": {}}

func IsReservedClassField(id string) bool {
	_, ok := luauReservedClassFields[id]
	return ok
}

func IsReservedIdentifier(id string) bool {
	_, ok := reservedGlobalNames[id]
	return ok
}
