package render

import "rotor/internal/luau"

func endsWithIndexableExpressionInner(node luau.Expression) bool {
	if luau.IsIndexableExpression(node) {
		return true
	}
	switch n := node.(type) {
	case *luau.BinaryExpression:
		return endsWithIndexableExpressionInner(n.Right)
	case *luau.UnaryExpression:
		return endsWithIndexableExpressionInner(n.Expression)
	case *luau.IfExpression:
		return endsWithIndexableExpressionInner(n.Alternative)
	}
	return false
}

func lastExprOf(v luau.NodeOrList) (luau.Expression, bool) {
	switch x := v.(type) {
	case *luau.List[luau.Expression]:
		if x.Tail == nil {
			panic("empty expression list")
		}
		return x.Tail.Value, true
	case *luau.List[luau.WritableExpression]:
		if x.Tail == nil {
			panic("empty writable list")
		}
		return x.Tail.Value, true
	case *luau.List[luau.AnyIdentifier]:
		if x.Tail == nil {
			panic("empty identifier list")
		}
		return x.Tail.Value, true
	case luau.Expression:
		return x, true
	}
	return nil, false
}

func endsWithIndexableExpression(node luau.Statement) bool {
	switch n := node.(type) {
	case *luau.CallStatement:
		return true
	case *luau.VariableDeclaration:
		v := n.Right
		if v == nil {
			v = n.Left
		}
		if e, ok := lastExprOf(v); ok {
			return endsWithIndexableExpressionInner(e)
		}
	case *luau.Assignment:
		v := n.Right
		if v == nil {
			v = n.Left
		}
		if e, ok := lastExprOf(v); ok {
			return endsWithIndexableExpressionInner(e)
		}
	}
	return false
}

func startsWithParenthesisInner(node luau.Expression) bool {
	switch n := node.(type) {
	case *luau.ParenthesizedExpression:
		return true
	case *luau.CallExpression:
		return startsWithParenthesisInner(n.Expression)
	case *luau.MethodCallExpression:
		return startsWithParenthesisInner(n.Expression)
	case *luau.PropertyAccessExpression:
		return startsWithParenthesisInner(n.Expression)
	case *luau.ComputedIndexExpression:
		return startsWithParenthesisInner(n.Expression)
	}
	return false
}

func startsWithParenthesis(node luau.Statement) bool {
	switch n := node.(type) {
	case *luau.CallStatement:
		switch e := n.Expression.(type) {
		case *luau.CallExpression:
			return startsWithParenthesisInner(e.Expression)
		case *luau.MethodCallExpression:
			return startsWithParenthesisInner(e.Expression)
		}
	case *luau.Assignment:
		switch l := n.Left.(type) {
		case *luau.List[luau.WritableExpression]:
			if l.Head == nil {
				panic("empty assignment left list")
			}
			return startsWithParenthesisInner(l.Head.Value)
		case luau.Expression:
			return startsWithParenthesisInner(l)
		}
	}
	return false
}

func getNextNonComment(s *RenderState) luau.Statement {
	listNode := s.peekListNode()
	if listNode == nil {
		return nil
	}
	next := listNode.Next
	for next != nil {
		if _, isComment := next.Value.(*luau.Comment); !isComment {
			break
		}
		next = next.Next
	}
	if next == nil {
		return nil
	}
	return next.Value
}

func getEnding(s *RenderState, node luau.Statement) string {
	next := getNextNonComment(s)
	if next != nil && endsWithIndexableExpression(node) && startsWithParenthesis(next) {
		return ";"
	}
	return ""
}
