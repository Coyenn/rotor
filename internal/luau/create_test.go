package luau

import "testing"

func TestAdoptSetsParent(t *testing.T) {
	left := ID("a")
	right := ID("b")
	bin := NewBinary(left, "+", right)
	if left.Parent() != Node(bin) || right.Parent() != Node(bin) {
		t.Error("children must be adopted")
	}
}

func TestAdoptClonesWhenAlreadyParented(t *testing.T) {
	shared := ID("x")
	first := NewBinary(shared, "+", ID("y"))
	second := NewBinary(shared, "-", ID("z"))
	if first.Left != Expression(shared) {
		t.Error("first use must keep the original node")
	}
	if second.Left == Expression(shared) {
		t.Error("second use must be a clone, not the original")
	}
	if second.Left.(*Identifier).Name != "x" {
		t.Error("clone must preserve fields")
	}
	if second.Left.Parent() != Node(second) {
		t.Error("clone must be adopted by the new parent")
	}
	if shared.Parent() != Node(first) {
		t.Error("original node's parent must be untouched")
	}
}

func TestListAdoption(t *testing.T) {
	a, b := ID("a"), ID("b")
	arr := NewArray(NewList[Expression](a, b))
	if a.Parent() != Node(arr) || b.Parent() != Node(arr) {
		t.Error("list elements must be adopted")
	}
	// reuse a in another list: the LIST NODE's value gets replaced by a clone
	l2 := NewList[Expression](a)
	arr2 := NewArray(l2)
	if l2.Head.Value == Expression(a) {
		t.Error("already-parented list element must be cloned")
	}
	if l2.Head.Value.Parent() != Node(arr2) {
		t.Error("cloned element must be adopted")
	}
}

func TestNumHandlesNegatives(t *testing.T) {
	n := Num(5)
	if lit, ok := n.(*NumberLiteral); !ok || lit.Value != "5" {
		t.Fatalf("Num(5) = %#v", n)
	}
	neg := Num(-3)
	un, ok := neg.(*UnaryExpression)
	if !ok || un.Operator != "-" {
		t.Fatalf("Num(-3) = %#v", neg)
	}
	if lit := un.Expression.(*NumberLiteral); lit.Value != "3" {
		t.Errorf("inner literal %q", lit.Value)
	}
}

func TestTempIDsUnique(t *testing.T) {
	a, b := TempID(""), TempID("foo")
	if a.ID == b.ID {
		t.Error("temp ids must be unique")
	}
	if b.Name != "foo" {
		t.Error("temp name")
	}
}
