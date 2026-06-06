package transformer

import (
	"errors"
	"strings"

	"rotor/internal/luau"
)

// CompilerVersion is the rbxtsc release rotor targets byte-for-byte. Upstream
// COMPILER_VERSION (Shared/constants.ts:9, the package.json version) is used
// ONLY in the header comment, which transformSourceFile.ts:225 prepends
// inside the transformer — not in compileFiles/renderAST.
const CompilerVersion = "3.0.0"

// ErrRuntimeLibNotSupported is returned by TransformSourceFile when a
// transform called State.RuntimeLib — emitting the runtime-library import
// (upstream createRuntimeLibImport) needs the rojo project layer.
var ErrRuntimeLibNotSupported = errors.New("runtime library imports not yet supported (Phase 4)")

// TransformSourceFile ports nodes/transformSourceFile.ts: transform the
// source file held by state into the final Luau statement list, ready for
// rendering.
func TransformSourceFile(s *State) (*luau.List[luau.Statement], error) {
	node := s.SourceFile

	// The file's module symbol maps to the identifier `exports`.
	symbol := s.Checker.GetSymbolAtLocation(node.AsNode())
	if symbol == nil {
		// Upstream assert: roblox-ts validates non-module files elsewhere.
		panic("transformer: source file has no module symbol")
	}
	s.SetModuleIDBySymbol(symbol, luau.GlobalID("exports"))

	statements := TransformStatementList(s, node.AsNode(), node.Statements.Nodes, nil)

	handleExports(s, node, symbol, statements)

	// moduleScripts must `return nil` if they do not export any values.
	// Phase 4: upstream consults rojoResolver.getRbxTypeFromFilePath on the
	// translated output path; until the rojo layer lands every compiled file
	// is a ModuleScript (true for all Phase 2 fixtures).
	const isModuleScript = true
	ensureModuleReturn(statements, isModuleScript)

	if s.UsesRuntimeLib {
		// Phase 4: state.createRuntimeLibImport(node) joins the header here.
		return nil, ErrRuntimeLibNotSupported
	}

	prependHeader(statements)

	return statements, nil
}

// ensureModuleReturn ports transformSourceFile.ts:213-220: append a plain
// `return nil` only when (a) the output file is a ModuleScript and (b) the
// last non-comment statement isn't already a return.
func ensureModuleReturn(statements *luau.List[luau.Statement], isModuleScript bool) {
	lastStatement := getLastNonCommentStatement(statements.Tail)
	if lastStatement == nil || lastStatement.Value.Kind() != luau.KindReturnStatement {
		if isModuleScript {
			statements.Push(luau.NewReturn(luau.Nil()))
		}
	}
}

// getLastNonCommentStatement ports transformSourceFile.ts:191-196: walk
// backwards past comments.
func getLastNonCommentStatement(listNode *luau.ListNode[luau.Statement]) *luau.ListNode[luau.Statement] {
	for listNode != nil && listNode.Value.Kind() == luau.KindComment {
		listNode = listNode.Prev
	}
	return listNode
}

// prependHeader ports transformSourceFile.ts:222-242: prepend the build
// header, hoisting any leading Luau directive comments (`--!strict`, sourced
// from leading `//!...` TS comments) above it. The header comment text starts
// with a space so it renders `-- Compiled with roblox-ts v3.0.0`.
func prependHeader(statements *luau.List[luau.Statement]) {
	headerStatements := luau.NewList[luau.Statement]()
	headerStatements.Push(luau.NewComment(" Compiled with roblox-ts v" + CompilerVersion))

	// Only a run of comments already at the very head of the list qualifies;
	// the scan stops at the first non-`!` comment or non-comment.
	directiveComments := luau.NewList[luau.Statement]()
	for statements.Head != nil {
		comment, ok := statements.Head.Value.(*luau.Comment)
		if !ok || !strings.HasPrefix(comment.Text, "!") {
			break
		}
		shifted, _ := statements.Shift()
		directiveComments.Push(shifted)
	}

	statements.UnshiftList(headerStatements)
	statements.UnshiftList(directiveComments)
}
