// Package conformance byte-compares rotor's output against rbxtsc 3.0.0
// goldens generated from roblox-ts's own upstream test corpus
// (testdata/conformance). See testdata/conformance/README.md for provenance
// and tools/oracle/conformance-oracle.ps1 for golden regeneration.
package conformance

// EnabledFixtures lists the golden-relative slash paths (e.g.
// "tests/array.spec.luau") whose rotor output must be byte-identical to
// testdata/conformance/golden/<path>. Phase 5 tasks append entries as
// transforms reach parity. A golden missing here is reported as skipped,
// never silently ignored. While this list is empty the test does not even
// compile the project, so it stays green regardless of transformer state.
var EnabledFixtures = []string{}
