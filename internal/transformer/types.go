package transformer

import (
	"rotor/tsgo/checker"
	"rotor/tsgo/jsnum"
)

// This file ports TSTransformer/util/types.ts: the isDefinitelyType /
// isPossiblyType combinators and the leaf predicates the first transform wave
// needs (truthiness, binary `+`, relational operators, logical chains).
//
// ts.TypeFlags -> tsgo mapping is 1:1 by name (checker.TypeFlags*); upstream
// helper methods on ts.Type map as:
//   type.getConstraint()          -> Checker.GetBaseConstraintOfType (TS public
//                                    Type.getConstraint is exactly that call)
//   type.isClassOrInterface()     -> getObjectFlags gate + ObjectFlagsClassOrInterface
//   type.getBaseTypes()           -> Checker.GetBaseTypes
//   type.isNumberLiteral()/.value -> Type.IsNumberLiteral + AsLiteralType().Value()
//   typeChecker.getTrueType()     -> identity = BooleanLiteral flag + value +
//                                    freshness (see IsBooleanLiteralType)

// TypeCheck ports `type TypeCheck = (type: ts.Type) => boolean` (types.ts L8).
// Go cannot compare func values, but isPossiblyTypeInner must reproduce
// upstream's `callbacks.length === 1 && callbacks[0] === isUndefinedType`
// identity test, so the predicate carries an explicit marker instead.
type TypeCheck struct {
	check          func(t *checker.Type) bool
	undefinedCheck bool
}

// Check runs the predicate on t.
func (tc TypeCheck) Check(t *checker.Type) bool { return tc.check(t) }

// isClassOrInterface ports ts.Type.isClassOrInterface():
// `getObjectFlags(this) & ObjectFlags.ClassOrInterface`, where getObjectFlags
// yields 0 unless `type.flags & TypeFlags.ObjectFlagsType`.
func isClassOrInterface(t *checker.Type) bool {
	return t.Flags()&checker.TypeFlagsObjectFlagsType != 0 &&
		t.ObjectFlags()&checker.ObjectFlagsClassOrInterface != 0
}

// getRecursiveBaseTypes ports types.ts getRecursiveBaseTypes (L10-23): all
// base types of a class/interface, transitively.
func getRecursiveBaseTypes(s *State, t *checker.Type) []*checker.Type {
	var result []*checker.Type
	var inner func(t *checker.Type)
	inner = func(t *checker.Type) {
		for _, baseType := range s.Checker.GetBaseTypes(t) {
			result = append(result, baseType)
			if isClassOrInterface(baseType) {
				inner(baseType)
			}
		}
	}
	inner(t)
	return result
}

// constraintOrSelf ports `type.getConstraint() ?? type` (types.ts L39/L69).
func constraintOrSelf(s *State, t *checker.Type) *checker.Type {
	if constraint := s.Checker.GetBaseConstraintOfType(t); constraint != nil {
		return constraint
	}
	return t
}

// isDefinitelyTypeInner ports types.ts L25-36: union -> EVERY member must
// match, intersection -> SOME member, class/interface -> base types may match,
// leaf -> SOME callback matches.
func isDefinitelyTypeInner(s *State, t *checker.Type, callbacks []TypeCheck) bool {
	if t.IsUnion() {
		for _, member := range t.Types() {
			if !isDefinitelyTypeInner(s, member, callbacks) {
				return false
			}
		}
		return true
	} else if t.IsIntersection() {
		for _, member := range t.Types() {
			if isDefinitelyTypeInner(s, member, callbacks) {
				return true
			}
		}
		return false
	}
	if isClassOrInterface(t) {
		for _, baseType := range getRecursiveBaseTypes(s, t) {
			if isDefinitelyTypeInner(s, baseType, callbacks) {
				return true
			}
		}
	}
	for _, cb := range callbacks {
		if cb.Check(t) {
			return true
		}
	}
	return false
}

// IsDefinitelyType ports types.ts isDefinitelyType (L38-40): does the type
// (or its constraint) certainly satisfy one of the predicates?
func IsDefinitelyType(s *State, t *checker.Type, callbacks ...TypeCheck) bool {
	return isDefinitelyTypeInner(s, constraintOrSelf(s, t), callbacks)
}

// isPossiblyTypeInner ports types.ts L42-66: union AND intersection -> SOME
// member; unconstrained type variables / any / unknown are possibly anything;
// the rbxts `defined` type is possibly everything except `undefined` when
// that is the sole query.
func isPossiblyTypeInner(s *State, t *checker.Type, callbacks []TypeCheck) bool {
	if t.Flags()&checker.TypeFlagsUnionOrIntersection != 0 {
		for _, member := range t.Types() {
			if isPossiblyTypeInner(s, member, callbacks) {
				return true
			}
		}
		return false
	}
	if isClassOrInterface(t) {
		for _, baseType := range getRecursiveBaseTypes(s, t) {
			if isPossiblyTypeInner(s, baseType, callbacks) {
				return true
			}
		}
	}

	// type variable without constraint, any, or unknown
	if t.Flags()&(checker.TypeFlagsTypeVariable|checker.TypeFlagsAnyOrUnknown) != 0 {
		return true
	}

	// defined type
	if isDefinedType(s, t) {
		if len(callbacks) == 1 && callbacks[0].undefinedCheck {
			// if only matching undefined, then defined means not possible
			return false
		}
		return true
	}

	for _, cb := range callbacks {
		if cb.Check(t) {
			return true
		}
	}
	return false
}

// IsPossiblyType ports types.ts isPossiblyType (L68-70): could the type (or
// its constraint) satisfy one of the predicates?
func IsPossiblyType(s *State, t *checker.Type, callbacks ...TypeCheck) bool {
	return isPossiblyTypeInner(s, constraintOrSelf(s, t), callbacks)
}

// isDefinedType ports types.ts isDefinedType (L72-81): detects the rbxts
// `defined` type — flags EXACTLY equal to Object with no properties, call or
// construct signatures, and no number/string index types.
func isDefinedType(s *State, t *checker.Type) bool {
	return t.Flags() == checker.TypeFlagsObject &&
		len(s.Checker.GetPropertiesOfType(t)) == 0 &&
		len(s.Checker.GetCallSignatures(t)) == 0 &&
		len(s.Checker.GetConstructSignatures(t)) == 0 &&
		s.Checker.GetNumberIndexType(t) == nil &&
		s.Checker.GetStringIndexType(t) == nil
}

// ---------------------------------------------------------------------------
// Leaf predicates (types.ts L83-197; first-wave subset — the macro-symbol
// predicates isArrayType/isSetType/isMapType/... land with their consumers)
// ---------------------------------------------------------------------------

// IsAnyType ports isAnyType (L83-85): identity with the checker's intrinsic
// `any` type.
func IsAnyType(s *State) TypeCheck {
	return TypeCheck{check: func(t *checker.Type) bool {
		return t == s.Checker.GetAnyType()
	}}
}

func isBooleanTypeCheck(t *checker.Type) bool {
	return t.Flags()&(checker.TypeFlagsBoolean|checker.TypeFlagsBooleanLiteral) != 0
}

// IsBooleanType ports isBooleanType (L87-89).
var IsBooleanType = TypeCheck{check: isBooleanTypeCheck}

// IsBooleanLiteralType ports isBooleanLiteralType (L91-99). Upstream compares
// identity against typeChecker.getTrueType()/getFalseType() — the FRESH
// true/false types. The checker creates exactly four boolean literal types
// (fresh/regular x true/false) and only the fresh ones satisfy
// `freshType == self`, so identity is equivalent to value + freshness (the s
// parameter mirrors the upstream signature; tsgo needs no checker access
// here). Non-literals fall back to isBooleanType, so plain `boolean` counts
// as possibly-false.
func IsBooleanLiteralType(s *State, value bool) TypeCheck {
	_ = s
	return TypeCheck{check: func(t *checker.Type) bool {
		if t.Flags()&checker.TypeFlagsBooleanLiteral != 0 {
			lt := t.AsLiteralType()
			return lt.Value() == value && lt.FreshType() == t
		}
		return isBooleanTypeCheck(t)
	}}
}

func isNumberTypeCheck(t *checker.Type) bool {
	return t.Flags()&(checker.TypeFlagsNumber|checker.TypeFlagsNumberLike|checker.TypeFlagsNumberLiteral) != 0
}

// IsNumberType ports isNumberType (L101-103).
var IsNumberType = TypeCheck{check: isNumberTypeCheck}

// IsNumberLiteralType ports isNumberLiteralType (L105-112): a number literal
// type matches by value; any other number-ish type matches too (non-literal
// fallback).
func IsNumberLiteralType(value jsnum.Number) TypeCheck {
	return TypeCheck{check: func(t *checker.Type) bool {
		if t.IsNumberLiteral() {
			return t.AsLiteralType().Value() == value
		}
		return isNumberTypeCheck(t)
	}}
}

func isNaNTypeCheck(t *checker.Type) bool {
	return isNumberTypeCheck(t) && !t.IsNumberLiteral()
}

// IsNaNType ports isNaNType (L114-116): number-ish AND not a number literal
// (a literal has a known value, which is never NaN; there is no distinct NaN
// type in the checker).
var IsNaNType = TypeCheck{check: isNaNTypeCheck}

func isStringTypeCheck(t *checker.Type) bool {
	return t.Flags()&(checker.TypeFlagsString|checker.TypeFlagsStringLike|checker.TypeFlagsStringLiteral) != 0
}

// IsStringType ports isStringType (L118-120).
var IsStringType = TypeCheck{check: isStringTypeCheck}

// IsObjectType ports isObjectType (L181-183).
var IsObjectType = TypeCheck{check: func(t *checker.Type) bool {
	return t.Flags()&checker.TypeFlagsObject != 0
}}

func isUndefinedTypeCheck(t *checker.Type) bool {
	return t.Flags()&(checker.TypeFlagsUndefined|checker.TypeFlagsVoid) != 0
}

// IsUndefinedType ports isUndefinedType (L185-187). Its undefinedCheck marker
// drives the `defined`-type special case in isPossiblyTypeInner.
var IsUndefinedType = TypeCheck{check: isUndefinedTypeCheck, undefinedCheck: true}

// isTemplateLiteralType ports typeGuards.ts isTemplateLiteralType (L19-21:
// `"texts" in type && "types" in type && flags & TemplateLiteral`); in tsgo
// the flag alone identifies *TemplateLiteralType.
func isTemplateLiteralType(t *checker.Type) bool {
	return t.Flags()&checker.TypeFlagsTemplateLiteral != 0
}

func isEmptyStringTypeCheck(t *checker.Type) bool {
	if t.IsStringLiteral() {
		return t.AsLiteralType().Value() == ""
	}
	if isTemplateLiteralType(t) {
		texts := t.AsTemplateLiteralType().Texts()
		for _, text := range texts {
			if len(text) != 0 {
				return false
			}
		}
		return true
	}
	return isStringTypeCheck(t)
}

// IsEmptyStringType ports isEmptyStringType (L189-197): string literal "",
// template literal type with all-empty texts (`texts.length === 0 ||
// texts.every(v => v.length === 0)`), or any non-literal string-ish type.
var IsEmptyStringType = TypeCheck{check: isEmptyStringTypeCheck}
