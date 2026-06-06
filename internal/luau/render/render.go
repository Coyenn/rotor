package render

import "rotor/internal/luau"

// Render dispatches on node kind. Cases filled in by Tasks 13/14.
func Render(s *RenderState, node luau.Node) string {
	panic("render: no renderer for " + node.Kind().String())
}
