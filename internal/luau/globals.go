package luau

// reservedGlobalNames contains exactly the top-level keys of upstream
// globals.ts (LuauAST/impl/globals.ts). IsReservedIdentifier depends on it.
var reservedGlobalNames = map[string]struct{}{
	"_G": {}, "TS": {}, "assert": {}, "bit32": {}, "coroutine": {}, "error": {},
	"exports": {}, "getmetatable": {}, "ipairs": {}, "next": {}, "pairs": {},
	"pcall": {}, "require": {}, "script": {}, "select": {}, "self": {},
	"setmetatable": {}, "string": {}, "super": {}, "table": {}, "utf8": {},
	"math": {}, "tostring": {}, "type": {}, "typeof": {}, "unpack": {}, "game": {},
}

func GlobalID(name string) *Identifier { return ID(name) }

func GlobalProperty(object, name string) *PropertyAccessExpression {
	return NewPropertyAccess(ID(object), name)
}
