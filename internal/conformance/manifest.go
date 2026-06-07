// Package conformance byte-compares rotor's output against rbxtsc 3.0.0
// goldens generated from roblox-ts's own upstream test corpus
// (testdata/conformance). See testdata/conformance/README.md for provenance
// and tools/oracle/conformance-oracle.ps1 for golden regeneration.
package conformance

// EnabledFixtures lists the golden-relative slash paths (e.g.
// "tests/array.spec.luau") whose rotor output must be byte-identical to
// testdata/conformance/golden/<path>.
var EnabledFixtures = []string{
	"helpers/rojo/isolated.luau",
	"helpers/rojo/noRojo.luau",
	"helpers/util/ClassWithInstanceFoo.luau",
	"helpers/util/ClassWithStaticFoo.luau",
	"helpers/util/nonModule.server.luau",
	"main.server.luau",
	"tests/assignment.spec.luau",
	"tests/binary.spec.luau",
	"tests/bitwise.spec.luau",
	"tests/class.spec.luau",
	"tests/decorator.spec.luau",
	"tests/destructure.spec.luau",
	"tests/enum.spec.luau",
	"tests/exportLet.spec.luau",
	"tests/function.spec.luau",
	"tests/generator.spec.luau",
	"tests/hoist.spec.luau",
	"tests/if.spec.luau",
	"tests/instanceof.spec.luau",
	"tests/literal.spec.luau",
	"tests/loop.spec.luau",
	"tests/macro_math.spec.luau",
	"tests/map.spec.luau",
	"tests/math.spec.luau",
	"tests/namespace.spec.luau",
	"tests/object.spec.luau",
	"tests/optional.spec.luau",
	"tests/promise.spec.luau",
	"tests/roact_spread.spec.luau",
	"tests/roblox.spec.luau",
	"tests/robloxEnum.spec.luau",
	"tests/set.spec.luau",
	"tests/string.spec.luau",
	"tests/switch.spec.luau",
	"tests/ternary.spec.luau",
	"tests/truthiness.spec.luau",
	"tests/try.spec.luau",
	"tests/tuple.spec.luau",
	"tests/type.spec.luau",
	"tests/typeof.spec.luau",
}

// DisabledFixtures records the remaining goldens that are still outside the
// currently supported Phase 5 fixture set, with the reason they are held back.
var DisabledFixtures = map[string]string{
	"tests/array.spec.luau":    "byte mismatch against the upstream golden",
	"tests/delete.spec.luau":   "compile emits unsupported DeleteExpression diagnostics",
	"tests/roact.spec.luau":    "byte mismatch against the upstream golden",
	"tests/template.spec.luau": "compile emits unsupported TaggedTemplateExpression diagnostics",
}
