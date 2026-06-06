package luau

import "testing"

func TestRangeGuards(t *testing.T) {
	if !IsExpression(ID("x")) || !IsIndexableExpression(ID("x")) {
		t.Error("identifier guards")
	}
	if IsStatement(ID("x")) || IsExpression(NewBreak()) {
		t.Error("category confusion")
	}
	if !IsStatement(NewBreak()) || !IsField(NewMapField(Str("k"), Nil())) {
		t.Error("statement/field guards")
	}
}

func TestCompositeGuards(t *testing.T) {
	if !IsSimple(Str("s")) || !IsSimple(TempID("")) || IsSimple(NewArray(NewList[Expression]())) {
		t.Error("IsSimple")
	}
	if !IsSimplePrimitive(Nil()) || IsSimplePrimitive(ID("x")) {
		t.Error("IsSimplePrimitive")
	}
	if !IsTable(NewSet(NewList[Expression]())) || IsTable(Str("s")) {
		t.Error("IsTable")
	}
	if !IsFinalStatement(NewBreak()) || !IsFinalStatement(NewContinue()) || IsFinalStatement(NewDo(NewList[Statement]())) {
		t.Error("IsFinalStatement")
	}
	c := NewCall(ID("f"), NewList[Expression]())
	if !IsCall(c) || !IsCall(NewMethodCall("m", ID("o"), NewList[Expression]())) {
		t.Error("IsCall")
	}
	if !IsFunctionLike(NewFunctionExpression(NewList[AnyIdentifier](), false, NewList[Statement]())) {
		t.Error("IsFunctionLike")
	}
	if !HasStatements(NewDo(NewList[Statement]())) || HasStatements(NewBreak()) {
		t.Error("HasStatements")
	}
	if !IsExpressionWithPrecedence(NewBinary(ID("a"), "+", ID("b"))) || IsExpressionWithPrecedence(ID("a")) {
		t.Error("IsExpressionWithPrecedence")
	}
}
