package luau

import "testing"

// placeholder until nodes.go lands (Task 8); Task 8 deletes this and rewrites
// these tests on *Identifier.
type testNode struct {
	base
	v int
}

func (*testNode) Kind() SyntaxKind     { return KindIdentifier }
func (n *testNode) shallowClone() Node { c := *n; return &c }

func tn(v int) *testNode { return &testNode{v: v} }

func values(l *List[*testNode]) []int {
	out := []int{}
	l.ForEach(func(n *testNode) { out = append(out, n.v) })
	return out
}

func eq(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestListPushShiftUnshift(t *testing.T) {
	l := NewList[*testNode]()
	if !l.IsEmpty() {
		t.Fatal("new list not empty")
	}
	l.Push(tn(2))
	l.Push(tn(3))
	l.Unshift(tn(1))
	if got := values(l); !eq(got, []int{1, 2, 3}) {
		t.Fatalf("got %v", got)
	}
	if l.Size() != 3 {
		t.Fatalf("size %d", l.Size())
	}
	first, ok := l.Shift()
	if !ok || first.v != 1 {
		t.Fatalf("shift got %v %v", first, ok)
	}
	if got := values(l); !eq(got, []int{2, 3}) {
		t.Fatalf("after shift got %v", got)
	}
}

func TestListPushListMarksReadonly(t *testing.T) {
	a := NewList(tn(1), tn(2))
	b := NewList(tn(3), tn(4))
	a.PushList(b)
	if got := values(a); !eq(got, []int{1, 2, 3, 4}) {
		t.Fatalf("got %v", got)
	}
	if !b.ReadOnly {
		t.Error("source list must be marked readonly after PushList")
	}
	defer func() {
		if recover() == nil {
			t.Error("pushing to readonly list must panic")
		}
	}()
	b.Push(tn(5))
}

func TestListUnshiftList(t *testing.T) {
	a := NewList(tn(3), tn(4))
	b := NewList(tn(1), tn(2))
	a.UnshiftList(b)
	if got := values(a); !eq(got, []int{1, 2, 3, 4}) {
		t.Fatalf("got %v", got)
	}
}

func TestListSomeEvery(t *testing.T) {
	l := NewList(tn(1), tn(2), tn(3))
	if !l.Some(func(n *testNode) bool { return n.v == 2 }) {
		t.Error("Some failed")
	}
	if l.Every(func(n *testNode) bool { return n.v < 3 }) {
		t.Error("Every failed")
	}
}
