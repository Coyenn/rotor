package luau

type ListNode[T Node] struct {
	Prev, Next *ListNode[T]
	Value      T
}

type List[T Node] struct {
	Head, Tail *ListNode[T]
	ReadOnly   bool
}

func (*List[T]) nodeOrList() {}

// AnyList lets union fields (NodeOrList) be tested for list-ness without
// knowing the element type. Mirrors upstream luau.list.isList.
type AnyList interface {
	NodeOrList
	anyList()
}

func (*List[T]) anyList() {}

func IsList(v NodeOrList) bool {
	_, ok := v.(AnyList)
	return ok
}

func NewList[T Node](values ...T) *List[T] {
	l := &List[T]{}
	for _, v := range values {
		l.Push(v)
	}
	return l
}

func (l *List[T]) assertWritable() {
	if l.ReadOnly {
		panic("list is readonly")
	}
}

func (l *List[T]) Push(value T) {
	l.assertWritable()
	node := &ListNode[T]{Value: value}
	if l.Tail != nil {
		l.Tail.Next = node
		node.Prev = l.Tail
	} else {
		l.Head = node
	}
	l.Tail = node
}

func (l *List[T]) PushList(other *List[T]) {
	l.assertWritable()
	other.assertWritable()
	other.ReadOnly = true
	if other.Head != nil && other.Tail != nil {
		if l.Head != nil {
			l.Tail.Next = other.Head
			other.Head.Prev = l.Tail
			l.Tail = other.Tail
		} else {
			l.Head = other.Head
			l.Tail = other.Tail
		}
	}
}

func (l *List[T]) Shift() (T, bool) {
	l.assertWritable()
	var zero T
	if l.Head == nil {
		return zero, false
	}
	head := l.Head
	if head.Next != nil {
		l.Head = head.Next
		head.Next.Prev = nil
	} else {
		l.Head, l.Tail = nil, nil
	}
	return head.Value, true
}

func (l *List[T]) Unshift(value T) {
	l.assertWritable()
	node := &ListNode[T]{Value: value}
	if l.Head != nil {
		l.Head.Prev = node
		node.Next = l.Head
	} else {
		l.Tail = node
	}
	l.Head = node
}

func (l *List[T]) UnshiftList(other *List[T]) {
	l.assertWritable()
	other.assertWritable()
	other.ReadOnly = true
	if other.Head != nil && other.Tail != nil {
		if l.Head != nil {
			l.Head.Prev = other.Tail
			other.Tail.Next = l.Head
			l.Head = other.Head
		} else {
			l.Head = other.Head
			l.Tail = other.Tail
		}
	}
}

func (l *List[T]) IsEmpty() bool    { return l.Head == nil }
func (l *List[T]) IsNonEmpty() bool { return l.Head != nil }

func (l *List[T]) ForEach(f func(T)) {
	for n := l.Head; n != nil; n = n.Next {
		f(n.Value)
	}
}

func (l *List[T]) ForEachNode(f func(*ListNode[T])) {
	for n := l.Head; n != nil; n = n.Next {
		f(n)
	}
}

func (l *List[T]) ToSlice() []T {
	out := []T{}
	l.ForEach(func(v T) { out = append(out, v) })
	return out
}

func (l *List[T]) Some(f func(T) bool) bool {
	for n := l.Head; n != nil; n = n.Next {
		if f(n.Value) {
			return true
		}
	}
	return false
}

func (l *List[T]) Every(f func(T) bool) bool {
	for n := l.Head; n != nil; n = n.Next {
		if !f(n.Value) {
			return false
		}
	}
	return true
}

func (l *List[T]) Size() int {
	size := 0
	for n := l.Head; n != nil; n = n.Next {
		size++
	}
	return size
}

// Clone makes a list of shallow-cloned elements (upstream list.clone).
func (l *List[T]) Clone() *List[T] {
	out := NewList[T]()
	l.ForEach(func(v T) { out.Push(v.shallowClone().(T)) })
	return out
}
