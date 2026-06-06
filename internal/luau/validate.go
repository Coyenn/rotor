package luau

import "regexp"

var luauReservedKeywords = map[string]struct{}{
	"and": {}, "break": {}, "do": {}, "else": {}, "elseif": {}, "end": {},
	"false": {}, "for": {}, "function": {}, "if": {}, "in": {}, "local": {},
	"nil": {}, "not": {}, "or": {}, "repeat": {}, "return": {}, "then": {},
	"true": {}, "until": {}, "while": {},
}

var luauIdentifierRegex = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func IsValidIdentifier(id string) bool {
	if _, reserved := luauReservedKeywords[id]; reserved {
		return false
	}
	return luauIdentifierRegex.MatchString(id)
}

var (
	decimalLiteralRegex     = regexp.MustCompile(`^(?:\d[\d_]*(?:\.[\d_]*)?|\.\d[\d_]*)(?:[eE][+-]?_*\d[\d_]*)?$`)
	binaryLiteralRegex      = regexp.MustCompile(`^0_*[bB]_*[01][01_]*$`)
	hexadecimalLiteralRegex = regexp.MustCompile(`^0_*[xX]_*[\da-fA-F][\da-fA-F_]*$`)
)

func IsValidNumberLiteral(text string) bool {
	return decimalLiteralRegex.MatchString(text) ||
		binaryLiteralRegex.MatchString(text) ||
		hexadecimalLiteralRegex.MatchString(text)
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
