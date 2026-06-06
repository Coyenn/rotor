package transformer

import (
	"rotor/internal/luau"
	"rotor/tsgo/ast"
)

// ensureTransformOrder ports util/ensureTransformOrder.ts: transform sibling
// expressions (binary operands, call args, array elements, template spans)
// while guaranteeing they execute in source order. Each node's transform is
// captured; when a LATER sibling produced prerequisite statements, every
// earlier expression that could observe those statements' side effects is
// pinned into a temp (`exp`). Expressions that cannot change value —
// primitives, temps, and reads of non-mutable (const) identifiers — stay
// inline.
func ensureTransformOrder(s *State, nodes []*ast.Node) []luau.Expression {
	return ensureTransformOrderWith(s, nodes, TransformExpression)
}

// ensureTransformOrderWith is ensureTransformOrder with an explicit per-node
// transformer (upstream's optional third parameter).
func ensureTransformOrderWith(s *State, nodes []*ast.Node, transformer func(*State, *ast.Node) luau.Expression) []luau.Expression {
	type expressionInfo struct {
		expression luau.Expression
		prereqs    *luau.List[luau.Statement]
	}

	infos := make([]expressionInfo, len(nodes))
	for i, node := range nodes {
		expression, prereqs := s.Capture(func() luau.Expression { return transformer(s, node) })
		infos[i] = expressionInfo{expression, prereqs}
	}

	// findLastIndex(expressionInfoList, info => !isEmpty(info.prereqs))
	lastIndexWithPrereqs := -1
	for i := len(infos) - 1; i >= 0; i-- {
		if infos[i].prereqs.IsNonEmpty() {
			lastIndexWithPrereqs = i
			break
		}
	}

	result := make([]luau.Expression, 0, len(infos))
	for i, info := range infos {
		s.PrereqList(info.prereqs)

		isConstVar := false
		if exp := nodes[i]; ast.IsIdentifier(exp) {
			if symbol := s.Checker.GetSymbolAtLocation(exp); symbol != nil && !IsSymbolMutable(s, symbol) {
				isConstVar = true
			}
		}

		if i < lastIndexWithPrereqs &&
			!luau.IsSimplePrimitive(info.expression) &&
			info.expression.Kind() != luau.KindTemporaryIdentifier &&
			!isConstVar {
			result = append(result, s.PushToVar(info.expression, "exp"))
		} else {
			result = append(result, info.expression)
		}
	}
	return result
}
