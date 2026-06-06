package transformer

import (
	"rotor/internal/luau"
	"rotor/tsgo/ast"
	"rotor/tsgo/scanner"
)

// getText ports ts.Node.getText(): the raw source slice from the token start
// (leading trivia skipped) to the node end.
func getText(s *State, node *ast.Node) string {
	return scanner.GetTextOfNodeFromSourceText(s.SourceFileText, node, false /*includeTrivia*/)
}

// transformNumericLiteral ports expressions/transformNumericLiteral.ts: the
// raw source text passes through untouched (hex stays `0xff`, binary stays
// `0b101`, separators stay `1_000_000`); the luau renderer owns any
// normalization.
func transformNumericLiteral(s *State, node *ast.Node) luau.Expression {
	return luau.NewNumberLiteral(getText(s, node))
}

// createStringFromLiteral ports util/createStringFromLiteral.ts: slice the
// raw getText() instead of using node.text, because the cooked text converts
// `\\n` to `\n` — escapes must be preserved verbatim.
func createStringFromLiteral(s *State, node *ast.Node) string {
	text := getText(s, node)
	switch node.Kind {
	case ast.KindStringLiteral, ast.KindNoSubstitutionTemplateLiteral:
		// ts.stripQuotes: drop the surrounding quote/backtick pair.
		return text[1 : len(text)-1]
	case ast.KindTemplateHead:
		// remove starting ` and ending ${
		return text[1 : len(text)-2]
	case ast.KindTemplateMiddle:
		// remove starting } and ending ${
		return text[1 : len(text)-2]
	case ast.KindTemplateTail:
		// remove starting } and ending `
		return text[1 : len(text)-1]
	}
	return text
}

// transformStringLiteral ports expressions/transformStringLiteral.ts. Quote
// style selection ("double" vs 'single') happens in the luau string renderer;
// the transformer stores the raw inner text.
func transformStringLiteral(s *State, node *ast.Node) luau.Expression {
	return luau.Str(createStringFromLiteral(s, node))
}

// transformInterpolatedStringPart ports nodes/transformInterpolatedStringPart.ts.
// The transformer passes the raw source text; the renderer escapes braces and
// newlines for the Luau backtick syntax (do not double-escape here).
func transformInterpolatedStringPart(s *State, node *ast.Node) *luau.InterpolatedStringPart {
	return luau.NewInterpolatedStringPart(createStringFromLiteral(s, node))
}

// transformNoSubstitutionTemplateLiteral ports
// expressions/transformNoSubstitutionTemplateLiteral.ts: backtick strings
// without `${}` stay as Luau interpolated strings (still valid Luau).
func transformNoSubstitutionTemplateLiteral(s *State, node *ast.Node) luau.Expression {
	parts := luau.NewList[luau.Node](luau.Node(transformInterpolatedStringPart(s, node)))
	return luau.NewInterpolatedString(parts)
}

// transformTemplateExpression ports expressions/transformTemplateExpression.ts:
// TS template strings ALWAYS become Luau InterpolatedString (no concatenation
// fallback in v3). Parts with empty cooked text are skipped; emitted text uses
// the raw getText() slices.
func transformTemplateExpression(s *State, node *ast.Node) luau.Expression {
	expression := node.AsTemplateExpression()
	parts := luau.NewList[luau.Node]()

	if len(expression.Head.Text()) > 0 {
		parts.Push(transformInterpolatedStringPart(s, expression.Head))
	}

	spans := expression.TemplateSpans.Nodes
	spanExpressions := make([]*ast.Node, len(spans))
	for i, span := range spans {
		spanExpressions[i] = span.AsTemplateSpan().Expression
	}
	orderedExpressions := transformExpressionsLeftToRight(s, spanExpressions)

	for i, span := range spans {
		parts.Push(orderedExpressions[i])
		literal := span.AsTemplateSpan().Literal
		if len(literal.Text()) > 0 {
			parts.Push(transformInterpolatedStringPart(s, literal))
		}
	}

	return luau.NewInterpolatedString(parts)
}

// transformArrayLiteralExpression ports
// expressions/transformArrayLiteralExpression.ts. Non-spread fast path only:
// inline `{ e1, e2, ... }`. The spread path (ArrayPointer + `#array` length
// temp + iterable builders) lands with the destructuring/spread task.
func transformArrayLiteralExpression(s *State, node *ast.Node) luau.Expression {
	elements := node.AsArrayLiteralExpression().Elements.Nodes
	for _, element := range elements {
		if ast.IsSpreadElement(element) {
			s.Diags.Add(DiagRotorNotYetSupported(element, "array spread elements"))
			return luau.NewNone()
		}
	}
	members := luau.NewList[luau.Expression](transformExpressionsLeftToRight(s, elements)...)
	return luau.NewArray(members)
}

// transformObjectLiteralExpression ports
// expressions/transformObjectLiteralExpression.ts: walk the properties
// building a MapPointer — inline `{ ... }` until a member's transform
// produces prereqs, then materialize a temp and emit per-field assignments.
// validateMethodAssignment (method-ness vs contextual type) lands with the
// function transforms.
func transformObjectLiteralExpression(s *State, node *ast.Node) luau.Expression {
	ptr := CreateMapPointer("object")
	for _, property := range node.AsObjectLiteralExpression().Properties.Nodes {
		switch property.Kind {
		case ast.KindPropertyAssignment:
			name := property.Name()
			if name.Kind == ast.KindPrivateIdentifier {
				s.Diags.Add(DiagNoPrivateIdentifier(name))
				continue
			}
			transformPropertyAssignment(s, ptr, name, property.AsPropertyAssignment().Initializer)
		case ast.KindShorthandPropertyAssignment:
			// `{ a }` — the name doubles as the initializer expression.
			name := property.Name()
			transformPropertyAssignment(s, ptr, name, name)
		case ast.KindSpreadAssignment:
			// table.clone fast path / generic-for copy: later task.
			s.Diags.Add(DiagRotorNotYetSupported(property, "object spread assignments"))
		case ast.KindMethodDeclaration:
			s.Diags.Add(DiagRotorNotYetSupported(property, "object literal method declarations"))
		default:
			// must be an AccessorDeclaration, which is banned
			s.Diags.Add(DiagNoGetterSetter(property))
		}
	}
	return ptr.Value
}

// transformPropertyAssignment ports transformObjectLiteralExpression.ts
// transformPropertyAssignment: if either side produced prereqs the inline
// table can no longer hold the field (evaluation order would change), so the
// pointer degrades to a temp and the key is pinned.
func transformPropertyAssignment(s *State, ptr *MapPointer, name *ast.Node, initializer *ast.Node) {
	left, leftPrereqs := s.Capture(func() luau.Expression { return transformPropertyName(s, name) })
	right, rightPrereqs := s.Capture(func() luau.Expression { return TransformExpression(s, initializer) })

	if leftPrereqs.IsNonEmpty() || rightPrereqs.IsNonEmpty() {
		DisableMapInline(s, ptr)
		s.PrereqList(leftPrereqs)
		left = s.PushToVar(left, "left")
	}

	s.PrereqList(rightPrereqs)

	AssignToMapPointer(s, ptr, left, right)
}

// transformPropertyName ports nodes/transformPropertyName.ts: a plain
// identifier key `a` becomes the string "a"; a computed key `[a]` transforms
// `name.expression`; string/number literal names transform directly. (The
// luau map renderer chooses `a = v` vs `["a"] = v` per index shape.)
func transformPropertyName(s *State, name *ast.Node) luau.Expression {
	if ast.IsIdentifier(name) {
		return luau.Str(name.Text())
	}
	if ast.IsComputedPropertyName(name) {
		return TransformExpression(s, name.AsComputedPropertyName().Expression)
	}
	return TransformExpression(s, name)
}

// transformParenthesizedExpression ports
// expressions/transformParenthesizedExpression.ts: unwrap, transform, and
// keep Luau parens only when the result is not simple.
func transformParenthesizedExpression(s *State, node *ast.Node) luau.Expression {
	expression := TransformExpression(s, SkipDownwards(node.AsParenthesizedExpression().Expression))
	if luau.IsSimple(expression) {
		return expression
	}
	return luau.NewParenthesized(expression)
}
