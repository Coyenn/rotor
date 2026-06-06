package transformer

import (
	"rotor/internal/luau"
)

// MapPointer ports util/pointer.ts MapPointer: Value is a *luau.Map while the
// table constructor is still being built inline, and becomes a
// *luau.TemporaryIdentifier after DisableMapInline materializes it into
// `local <name> = { ... }`.
type MapPointer struct {
	Name  string
	Value luau.Expression // *luau.Map | *luau.TemporaryIdentifier
}

// ArrayPointer ports util/pointer.ts ArrayPointer (same inline-until-disabled
// mechanics with a *luau.Array).
type ArrayPointer struct {
	Name  string
	Value luau.Expression // *luau.Array | *luau.TemporaryIdentifier
}

// CreateMapPointer ports createMapPointer: starts as an empty inline map
// literal.
func CreateMapPointer(name string) *MapPointer {
	return &MapPointer{Name: name, Value: luau.NewMap(luau.NewList[*luau.MapField]())}
}

// CreateArrayPointer ports createArrayPointer.
func CreateArrayPointer(name string) *ArrayPointer {
	return &ArrayPointer{Name: name, Value: luau.NewArray(luau.NewList[luau.Expression]())}
}

// AssignToMapPointer ports assignToMapPointer: while the pointer is still an
// inline map literal, push a MapField onto it; once materialized, emit
// `ptr[left] = right` as a prereq statement.
func AssignToMapPointer(s *State, ptr *MapPointer, left, right luau.Expression) {
	if m, ok := ptr.Value.(*luau.Map); ok {
		m.Fields.Push(luau.NewMapField(left, right))
	} else {
		s.Prereq(luau.NewAssignment(
			luau.NewComputedIndex(ptr.Value.(luau.IndexableExpression), left),
			"=",
			right,
		))
	}
}

// DisableMapInline ports disableMapInline: if the pointer is still the inline
// map literal, push it to a temp (`local <name> = { ...so far }`) so
// subsequent fields are emitted as assignments.
func DisableMapInline(s *State, ptr *MapPointer) {
	if _, ok := ptr.Value.(*luau.Map); ok {
		ptr.Value = s.PushToVar(ptr.Value, ptr.Name)
	}
}

// DisableArrayInline ports disableArrayInline.
func DisableArrayInline(s *State, ptr *ArrayPointer) {
	if _, ok := ptr.Value.(*luau.Array); ok {
		ptr.Value = s.PushToVar(ptr.Value, ptr.Name)
	}
}
