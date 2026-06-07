// Package conformance byte-compares rotor's output against rbxtsc 3.0.0
// goldens generated from roblox-ts's own upstream test corpus
// (testdata/conformance). See testdata/conformance/README.md for provenance
// and tools/oracle/conformance-oracle.ps1 for golden regeneration.
package conformance

// EnabledFixtures lists the golden-relative slash paths (e.g.
// "tests/array.spec.luau") whose rotor output must be byte-identical to
// testdata/conformance/golden/<path>. The list starts with fixtures that are
// known to compile on the current branch and expands as more transformer
// nodes reach parity.
var EnabledFixtures = []string{
	"helpers/util/ClassWithInstanceFoo.luau",
	"helpers/util/ClassWithStaticFoo.luau",
	"main.server.luau",
	"tests/literal.spec.luau",
	"tests/binary.spec.luau",
	"tests/type.spec.luau",
}
