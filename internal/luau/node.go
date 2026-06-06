package luau

// NodeOrList is the union of any AST node and any *List[T] — mirrors upstream
// fields typed `luau.X | luau.List<luau.Y>`.
type NodeOrList interface{ nodeOrList() }

type Node interface {
	NodeOrList
	Kind() SyntaxKind
	Parent() Node
	setParent(Node)
	shallowClone() Node
}

// Marker interfaces mirror upstream category types.
type Expression interface {
	Node
	expressionNode()
}

type IndexableExpression interface {
	Expression
	indexableNode()
}

type Statement interface {
	Node
	statementNode()
}

type FieldNode interface {
	Node
	fieldNode()
}

// AnyIdentifier = Identifier | TemporaryIdentifier
type AnyIdentifier interface {
	IndexableExpression
	anyIdentifierNode()
}

// WritableExpression = AnyIdentifier | PropertyAccessExpression | ComputedIndexExpression
type WritableExpression interface {
	Expression
	writableNode()
}

// TODO(Task 7): HasParameters mirrors the upstream HasParameters interface.
// It is commented out until List[T] is introduced in Task 7.
// type HasParameters interface {
// 	Node
// 	ParamData() (params *List[AnyIdentifier], hasDotDotDot bool)
// }

// base is embedded in every node struct.
type base struct{ parent Node }

func (b *base) Parent() Node     { return b.parent }
func (b *base) setParent(p Node) { b.parent = p }
func (b *base) nodeOrList()      {}
