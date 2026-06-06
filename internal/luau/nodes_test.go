package luau

import "testing"

func TestCategoryMarkers(t *testing.T) {
	var _ IndexableExpression = (*Identifier)(nil)
	var _ IndexableExpression = (*TemporaryIdentifier)(nil)
	var _ IndexableExpression = (*ComputedIndexExpression)(nil)
	var _ IndexableExpression = (*PropertyAccessExpression)(nil)
	var _ IndexableExpression = (*CallExpression)(nil)
	var _ IndexableExpression = (*MethodCallExpression)(nil)
	var _ IndexableExpression = (*ParenthesizedExpression)(nil)

	var _ Expression = (*NilLiteral)(nil)
	var _ Expression = (*MixedTable)(nil)

	var _ Statement = (*Assignment)(nil)
	var _ Statement = (*Comment)(nil)

	var _ FieldNode = (*MapField)(nil)
	var _ FieldNode = (*InterpolatedStringPart)(nil)

	var _ AnyIdentifier = (*Identifier)(nil)
	var _ AnyIdentifier = (*TemporaryIdentifier)(nil)

	var _ WritableExpression = (*Identifier)(nil)
	var _ WritableExpression = (*TemporaryIdentifier)(nil)
	var _ WritableExpression = (*PropertyAccessExpression)(nil)
	var _ WritableExpression = (*ComputedIndexExpression)(nil)

	var _ HasParameters = (*FunctionExpression)(nil)
	var _ HasParameters = (*FunctionDeclaration)(nil)
	var _ HasParameters = (*MethodDeclaration)(nil)
}

func TestKinds(t *testing.T) {
	if (&Identifier{}).Kind() != KindIdentifier {
		t.Error("Identifier kind")
	}
	if (&InterpolatedStringPart{}).Kind() != KindInterpolatedStringPart {
		t.Error("InterpolatedStringPart kind")
	}
}
