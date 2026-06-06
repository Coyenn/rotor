// ROTOR ADDITION — this file is NOT part of the typescript-go mirror.
//
// tools/mirror regenerates ./tsgo from the pinned upstream commit and deletes
// everything first, including this file; re-add it after re-mirroring. A
// forgotten re-add is caught immediately: rotor/internal/transformer fails to
// build without it.

package checker

import "rotor/tsgo/ast"

// GetTypeOfAssignmentPattern exposes getTypeOfAssignmentPattern (services.go)
// for rotor's destructuring transforms: the type of an
// ArrayLiteralExpression/ObjectLiteralExpression used as a destructuring
// assignment LHS (`[a, b] = exp`, `({ a } = exp)`). Mirrors the TypeScript
// checker API of the same name that roblox-ts consumes
// (transformArrayAssignmentPattern.ts L24, transformObjectAssignmentPattern.ts
// L25/L55), including strada's `|| errorType` fallback.
func (c *Checker) GetTypeOfAssignmentPattern(expr *ast.Node) *Type {
	if t := c.getTypeOfAssignmentPattern(expr); t != nil {
		return t
	}
	return c.errorType
}
