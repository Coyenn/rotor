package transformer

import (
	"strings"
	"testing"

	"rotor/internal/luau"
	"rotor/internal/luau/render"
	"rotor/tsgo/ast"
)

// renderMacroExpr renders a macro-produced expression by embedding it in a
// `local x = <expr>` statement (RenderAST runs the temp-id solver).
func renderMacroExpr(t *testing.T, expr luau.Expression) string {
	t.Helper()
	out := render.RenderAST(luau.NewList[luau.Statement](luau.NewVariableDeclaration(luau.ID("x"), expr)))
	return strings.TrimSuffix(strings.TrimPrefix(out, "local x = "), "\n")
}

// TestStringMacroEmits invokes every STRING_CALLBACKS entry (through the
// wrapComments-wrapped table, as installed by init) and checks the emitted
// shape: `size` is `#expression`; everything else is
// `string.X(expression, ...args)` with the base FIRST and the args appended
// verbatim (no offsetting — the Luau string API is 1-based). None may
// produce prereqs, so the comment markers never appear for String methods.
func TestStringMacroEmits(t *testing.T) {
	s := NewTestState()
	base := func() luau.Expression { return luau.ID("s") }
	tests := []struct {
		method string
		base   luau.Expression
		args   []luau.Expression
		want   string
	}{
		{"size", base(), nil, "#s"},
		// method-style call on a string LITERAL: the literal is just the
		// first call argument / unary operand — no parens needed
		// (oracle-pinned in 26_stringmacros: `#"abc"`, `string.sub("literal", 1, 3)`).
		{"size", luau.Str("abc"), nil, `#"abc"`},
		{"byte", base(), nil, "string.byte(s)"},
		{"byte", base(), []luau.Expression{luau.Num(1), luau.Num(2)}, "string.byte(s, 1, 2)"},
		{"find", base(), []luau.Expression{luau.Str("o"), luau.Num(2), luau.Bool(true)}, `string.find(s, "o", 2, true)`},
		{"format", luau.Str("%d/%s"), []luau.Expression{luau.Num(5), luau.Str("x")}, `string.format("%d/%s", 5, "x")`},
		{"gmatch", base(), []luau.Expression{luau.Str("%a+")}, `string.gmatch(s, "%a+")`},
		{"gsub", base(), []luau.Expression{luau.Str("l"), luau.Str("L"), luau.Num(1)}, `string.gsub(s, "l", "L", 1)`},
		{"lower", base(), nil, "string.lower(s)"},
		{"match", base(), []luau.Expression{luau.Str("(%a+)")}, `string.match(s, "(%a+)")`},
		{"rep", base(), []luau.Expression{luau.Num(3)}, "string.rep(s, 3)"},
		{"reverse", base(), nil, "string.reverse(s)"},
		{"split", base(), []luau.Expression{luau.Str(", ")}, `string.split(s, ", ")`},
		{"sub", luau.Str("xyz"), []luau.Expression{luau.Num(1), luau.Num(2)}, `string.sub("xyz", 1, 2)`},
		{"upper", base(), nil, "string.upper(s)"},
	}
	covered := map[string]bool{}
	for _, tt := range tests {
		covered[tt.method] = true
		macro := stringCallbacks[tt.method]
		if macro == nil {
			t.Fatalf("stringCallbacks[%q] missing", tt.method)
		}
		expr, prereqs := s.Capture(func() luau.Expression {
			return macro(s, nil, tt.base, tt.args)
		})
		if prereqs.IsNonEmpty() {
			t.Errorf("%s: string macros must not produce prereqs, got %d", tt.method, prereqs.Size())
		}
		if got := renderMacroExpr(t, expr); got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.method, got, tt.want)
		}
	}
	if len(covered) != len(stringCallbacks) {
		t.Errorf("covered %d of %d STRING_CALLBACKS entries", len(covered), len(stringCallbacks))
	}
}

// TestArrayLikeSizeMacro: ArrayLike.size emits `#expression` (the only
// ARRAY_LIKE_METHODS entry).
func TestArrayLikeSizeMacro(t *testing.T) {
	s := NewTestState()
	if len(arrayLikeMethods) != 1 {
		t.Fatalf("ARRAY_LIKE_METHODS has %d entries, want 1", len(arrayLikeMethods))
	}
	expr, prereqs := s.Capture(func() luau.Expression {
		return arrayLikeMethods["size"](s, nil, luau.ID("arr"), nil)
	})
	if prereqs.IsNonEmpty() {
		t.Errorf("ArrayLike.size must not produce prereqs")
	}
	if got := renderMacroExpr(t, expr); got != "#arr" {
		t.Errorf("ArrayLike.size: got %q, want %q", got, "#arr")
	}
}

// fakeMacro builds a PropertyCallMacro that optionally pushes the incoming
// base to a var first (the header-exempt push) and then emits n extra
// CallStatement prereqs.
func fakeMacro(pushBase bool, extra int) PropertyCallMacro {
	return func(s *State, node *ast.Node, expression luau.Expression, args []luau.Expression) luau.Expression {
		if pushBase {
			expression = s.PushToVarIfComplex(expression, "exp")
		}
		for i := 0; i < extra; i++ {
			s.Prereq(luau.NewCallStatement(luau.NewCall(luau.GlobalID("doWork"), luau.NewList[luau.Expression](expression))))
		}
		return expression
	}
}

// TestWrapComments checks the exact threshold and push-exempt rules of
// propertyCallMacros.ts wrapComments (L965-994): markers appear only when the
// macro produced MORE THAN ONE prereq statement after excluding a leading
// `local _exp = <base>` push of the incoming expression, and that push stays
// ABOVE the header comment.
func TestWrapComments(t *testing.T) {
	complexBase := func() luau.Expression { return luau.NewBinary(luau.ID("a"), "+", luau.ID("b")) }
	tests := []struct {
		name     string
		macro    PropertyCallMacro
		base     func() luau.Expression
		want     string // rendered prereq statements
		wantExpr string
	}{
		{
			name:     "zero prereqs, no markers",
			macro:    fakeMacro(false, 0),
			base:     func() luau.Expression { return luau.ID("v") },
			want:     "",
			wantExpr: "v",
		},
		{
			name:     "one prereq, no markers",
			macro:    fakeMacro(false, 1),
			base:     func() luau.Expression { return luau.ID("v") },
			want:     "doWork(v)\n",
			wantExpr: "v",
		},
		{
			name:  "two prereqs, markers",
			macro: fakeMacro(false, 2),
			base:  func() luau.Expression { return luau.ID("v") },
			want: `-- ▼ Test.method ▼
doWork(v)
doWork(v)
-- ▲ Test.method ▲
`,
			wantExpr: "v",
		},
		{
			name:     "base push only, exempt, no markers",
			macro:    fakeMacro(true, 0),
			base:     complexBase,
			want:     "local _exp = a + b\n",
			wantExpr: "_exp",
		},
		{
			name:  "base push + one prereq, still below threshold",
			macro: fakeMacro(true, 1),
			base:  complexBase,
			want: `local _exp = a + b
doWork(_exp)
`,
			wantExpr: "_exp",
		},
		{
			name:  "base push + two prereqs, push stays above header",
			macro: fakeMacro(true, 2),
			base:  complexBase,
			want: `local _exp = a + b
-- ▼ Test.method ▼
doWork(_exp)
doWork(_exp)
-- ▲ Test.method ▲
`,
			wantExpr: "_exp",
		},
		{
			name: "leading temp declaration NOT of the base counts toward the threshold",
			macro: func(s *State, node *ast.Node, expression luau.Expression, args []luau.Expression) luau.Expression {
				// a VariableDeclaration whose right is NOT pointer-identical
				// to the incoming base — wasExpressionPushed must reject it.
				id := s.PushToVar(luau.NewBinary(luau.ID("c"), "+", luau.ID("d")), "other")
				s.Prereq(luau.NewCallStatement(luau.NewCall(luau.GlobalID("doWork"), luau.NewList[luau.Expression](id))))
				return expression
			},
			base: func() luau.Expression { return luau.ID("v") },
			want: `-- ▼ Test.method ▼
local _other = c + d
doWork(_other)
-- ▲ Test.method ▲
`,
			wantExpr: "v",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewTestState()
			wrapped := wrapComments("Test.method", tt.macro)
			var expr luau.Expression
			prereqs := s.CaptureStatements(func() {
				expr = wrapped(s, nil, tt.base(), nil)
			})
			// render expression and prereqs together so the temp-id solver
			// names temps consistently with the declarations.
			prereqs.Push(luau.NewCallStatement(luau.NewCall(luau.GlobalID("use"), luau.NewList[luau.Expression](expr))))
			got := render.RenderAST(prereqs)
			want := tt.want + "use(" + tt.wantExpr + ")\n"
			if got != want {
				t.Errorf("rendered:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}

// TestArgumentsWithDefaults ports the argumentsWithDefaults contract
// (L126-157): simple-primitive args pass through untouched; other provided
// args are pushed to a hinted temp guarded by `if argN == nil then argN =
// default end`; omitted trailing args become the default expressions
// literally.
func TestArgumentsWithDefaults(t *testing.T) {
	t.Run("simple primitive passes through", func(t *testing.T) {
		s := NewTestState()
		arg := luau.Str("x")
		var args []luau.Expression
		prereqs := s.CaptureStatements(func() {
			args = argumentsWithDefaults(s, []luau.Expression{arg}, []luau.Expression{luau.Str(", ")})
		})
		if prereqs.IsNonEmpty() {
			t.Errorf("simple primitive must not produce prereqs")
		}
		if len(args) != 1 || args[0] != luau.Expression(arg) {
			t.Errorf("arg must pass through untouched")
		}
	})

	t.Run("non-primitive arg gets nil-guarded temp", func(t *testing.T) {
		s := NewTestState()
		var args []luau.Expression
		prereqs := s.CaptureStatements(func() {
			args = argumentsWithDefaults(s, []luau.Expression{luau.ID("sep")}, []luau.Expression{luau.Str(", ")})
		})
		if _, ok := args[0].(*luau.TemporaryIdentifier); !ok {
			t.Fatalf("arg must become a temp, got %T", args[0])
		}
		prereqs.Push(luau.NewCallStatement(luau.NewCall(luau.GlobalID("use"), luau.NewList(args...))))
		want := `local _sep = sep
if _sep == nil then
	_sep = ", "
end
use(_sep)
`
		if got := render.RenderAST(prereqs); got != want {
			t.Errorf("rendered:\n%s\nwant:\n%s", got, want)
		}
	})

	t.Run("missing trailing args filled with defaults literally", func(t *testing.T) {
		s := NewTestState()
		def := luau.Str(", ")
		var args []luau.Expression
		prereqs := s.CaptureStatements(func() {
			args = argumentsWithDefaults(s, nil, []luau.Expression{def})
		})
		if prereqs.IsNonEmpty() {
			t.Errorf("filling missing args must not produce prereqs")
		}
		if len(args) != 1 || args[0] != luau.Expression(def) {
			t.Errorf("missing arg must be the default expression itself")
		}
	})
}
