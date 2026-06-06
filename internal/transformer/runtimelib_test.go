package transformer

import (
	"testing"

	"rotor/internal/luau"
	"rotor/internal/luau/render"
	"rotor/internal/rojo"
)

func renderStatement(stmt luau.Statement) string {
	list := luau.NewList[luau.Statement]()
	list.Push(stmt)
	return render.RenderAST(list)
}

// End-to-end coverage of all three emission forms against rbxtsc oracle
// output lives in internal/compile/project_test.go; these pin the chain
// construction in isolation (multi-segment WaitForChild nesting and the
// runtimeLibRbxPath-presence branch keying).
func TestCreateRuntimeLibImportShapes(t *testing.T) {
	t.Run("game WaitForChild chain", func(t *testing.T) {
		s := NewTestState()
		s.Rojo = &RojoContext{RuntimeLibRbxPath: rojo.RbxPath{"ReplicatedStorage", "rbxts_include", "RuntimeLib"}}
		s.ProjectType = ProjectTypeGame
		got := renderStatement(s.CreateRuntimeLibImport(nil))
		want := "local TS = require(game:GetService(\"ReplicatedStorage\"):WaitForChild(\"rbxts_include\"):WaitForChild(\"RuntimeLib\"))\n"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("package _G[script]", func(t *testing.T) {
		s := NewTestState()
		s.Rojo = &RojoContext{} // nil RuntimeLibRbxPath = upstream undefined
		s.ProjectType = ProjectTypePackage
		got := renderStatement(s.CreateRuntimeLibImport(nil))
		want := "local TS = _G[script]\n"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}

// propertyAccessExpressionChain ports util/expressionChain.ts — the Model
// require chain (and Task 4's import chains) build on it.
func TestPropertyAccessExpressionChain(t *testing.T) {
	expr := propertyAccessExpressionChain(luau.GlobalID("script"), []string{"Parent", "include", "RuntimeLib"})
	list := luau.NewList[luau.Statement]()
	list.Push(luau.NewVariableDeclaration(luau.GlobalID("TS"), luau.NewCall(luau.GlobalID("require"), luau.NewList[luau.Expression](expr))))
	want := "local TS = require(script.Parent.include.RuntimeLib)\n"
	if got := render.RenderAST(list); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
