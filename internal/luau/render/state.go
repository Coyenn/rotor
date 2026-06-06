package render

import (
	"strconv"

	"rotor/internal/luau"
)

const indentCharacter = "\t"

type RenderState struct {
	indent         string
	seenTempNodes  map[int]string
	tempIDFallback int
	listNodesStack []*luau.ListNode[luau.Statement]
}

func NewRenderState() *RenderState {
	return &RenderState{seenTempNodes: map[int]string{}}
}

func (s *RenderState) pushIndent() { s.indent += indentCharacter }
func (s *RenderState) popIndent()  { s.indent = s.indent[len(indentCharacter):] }

// getTempName: solveTempIds should have populated seenTempNodes; the counter
// fallback mirrors upstream's safety net.
func (s *RenderState) getTempName(node *luau.TemporaryIdentifier) string {
	if name, ok := s.seenTempNodes[node.ID]; ok {
		return name
	}
	name := "_" + strconv.Itoa(s.tempIDFallback)
	s.tempIDFallback++
	s.seenTempNodes[node.ID] = name
	return name
}

func (s *RenderState) pushListNode(n *luau.ListNode[luau.Statement]) {
	s.listNodesStack = append(s.listNodesStack, n)
}

func (s *RenderState) peekListNode() *luau.ListNode[luau.Statement] {
	if len(s.listNodesStack) == 0 {
		return nil
	}
	return s.listNodesStack[len(s.listNodesStack)-1]
}

func (s *RenderState) popListNode() {
	s.listNodesStack = s.listNodesStack[:len(s.listNodesStack)-1]
}

func (s *RenderState) Indented(text string) string { return s.indent + text }

func (s *RenderState) Line(text string) string { return s.Indented(text) + "\n" }

// LineWithEnd is upstream state.line(text, endNode): appends `;` when needed
// to avoid Luau ambiguous-syntax errors.
func (s *RenderState) LineWithEnd(text string, endNode luau.Statement) string {
	return s.Indented(text) + getEnding(s, endNode) + "\n"
}

func (s *RenderState) Block(callback func() string) string {
	s.pushIndent()
	result := callback()
	s.popIndent()
	return result
}
