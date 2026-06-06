package diff

// EnabledFixtures lists the fixture basenames (without extension) whose rotor
// output must be byte-identical to testdata/diff/golden/<name>.luau.
// Tasks append to this list as transforms land. A fixture missing here is
// reported as skipped, never silently ignored.
var EnabledFixtures = []string{
	"01_literals",
	"02_variables",
	"03_arithmetic",
	"04_logic",
	"05_control",
	"06_globals",
	"07_access",
	"08_exports",
	"09_unary",
	"10_strings",
	"11_edge_numbers",
	"12_edge_strings",
	"13_mixed",
	"14_functions",
	"15_arrows",
	"16_destructuring",
	"19_closures",
}
