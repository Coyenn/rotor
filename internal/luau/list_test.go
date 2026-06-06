package luau

import "testing"

func id(name string) *Identifier { return &Identifier{Name: name} }

func values(l *List[*Identifier]) []string {
	out := []string{}
	l.ForEach(func(n *Identifier) { out = append(out, n.Name) })
	return out
}

func eq(a, b []string) bool {
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
	l := NewList[*Identifier]()
	if !l.IsEmpty() {
		t.Fatal("new list not empty")
	}
	l.Push(id("2"))
	l.Push(id("3"))
	l.Unshift(id("1"))
	if got := values(l); !eq(got, []string{"1", "2", "3"}) {
		t.Fatalf("got %v", got)
	}
	if l.Size() != 3 {
		t.Fatalf("size %d", l.Size())
	}
	first, ok := l.Shift()
	if !ok || first.Name != "1" {
		t.Fatalf("shift got %v %v", first, ok)
	}
	if got := values(l); !eq(got, []string{"2", "3"}) {
		t.Fatalf("after shift got %v", got)
	}
}

func TestListPushListMarksReadonly(t *testing.T) {
	a := NewList(id("1"), id("2"))
	b := NewList(id("3"), id("4"))
	a.PushList(b)
	if got := values(a); !eq(got, []string{"1", "2", "3", "4"}) {
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
	b.Push(id("5"))
}

func TestListUnshiftList(t *testing.T) {
	a := NewList(id("3"), id("4"))
	b := NewList(id("1"), id("2"))
	a.UnshiftList(b)
	if got := values(a); !eq(got, []string{"1", "2", "3", "4"}) {
		t.Fatalf("got %v", got)
	}
}

func TestListSpliceEmptySource(t *testing.T) {
	// splicing in an empty list is a no-op, but the source must still be
	// marked readonly
	a := NewList(id("1"), id("2"))
	emptyPush := NewList[*Identifier]()
	a.PushList(emptyPush)
	if got := values(a); !eq(got, []string{"1", "2"}) {
		t.Fatalf("after PushList(empty) got %v", got)
	}
	if !emptyPush.ReadOnly {
		t.Error("empty source must be marked readonly after PushList")
	}

	b := NewList(id("1"), id("2"))
	emptyUnshift := NewList[*Identifier]()
	b.UnshiftList(emptyUnshift)
	if got := values(b); !eq(got, []string{"1", "2"}) {
		t.Fatalf("after UnshiftList(empty) got %v", got)
	}
	if !emptyUnshift.ReadOnly {
		t.Error("empty source must be marked readonly after UnshiftList")
	}
}

func TestListSpliceOntoEmptyDestination(t *testing.T) {
	dst := NewList[*Identifier]()
	dst.PushList(NewList(id("1"), id("2")))
	if got := values(dst); !eq(got, []string{"1", "2"}) {
		t.Fatalf("PushList onto empty got %v", got)
	}
	if dst.Tail == nil || dst.Tail.Value.Name != "2" {
		t.Fatal("PushList onto empty must set Tail")
	}

	dst2 := NewList[*Identifier]()
	dst2.UnshiftList(NewList(id("1"), id("2")))
	if got := values(dst2); !eq(got, []string{"1", "2"}) {
		t.Fatalf("UnshiftList onto empty got %v", got)
	}
	if dst2.Tail == nil || dst2.Tail.Value.Name != "2" {
		t.Fatal("UnshiftList onto empty must set Tail")
	}
}

func TestListSomeEvery(t *testing.T) {
	l := NewList(id("1"), id("2"), id("3"))
	if !l.Some(func(n *Identifier) bool { return n.Name == "2" }) {
		t.Error("Some failed")
	}
	if l.Every(func(n *Identifier) bool { return n.Name < "3" }) {
		t.Error("Every failed")
	}
}
