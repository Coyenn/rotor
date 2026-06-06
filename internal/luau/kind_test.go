package luau

import "testing"

func TestKindOrdering(t *testing.T) {
	// category ranges must mirror upstream enums.ts exactly
	if FirstIndexableExpression != KindIdentifier || LastIndexableExpression != KindParenthesizedExpression {
		t.Error("indexable expression range wrong")
	}
	if FirstExpression != KindIdentifier || LastExpression != KindMixedTable {
		t.Error("expression range wrong")
	}
	if FirstStatement != KindAssignment || LastStatement != KindComment {
		t.Error("statement range wrong")
	}
	if FirstField != KindMapField || LastField != KindInterpolatedStringPart {
		t.Error("field range wrong")
	}
}

func TestKindName(t *testing.T) {
	cases := map[SyntaxKind]string{
		KindIdentifier:              "Identifier",
		KindParenthesizedExpression: "ParenthesizedExpression",
		KindSet:                     "Set",
		KindAssignment:              "Assignment",
		KindComment:                 "Comment",
		KindMapField:                "MapField",
		KindInterpolatedString:      "InterpolatedString",
		KindNumericForStatement:     "NumericForStatement",
	}
	for k, want := range cases {
		if got := k.String(); got != want {
			t.Errorf("kind %d String() = %q, want %q", k, got, want)
		}
	}
}
