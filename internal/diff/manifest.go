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
	"17_forof",
	"18_switch",
	"19_closures",
	"20_mixed2b",
	"21a_util",
	"21b_main",
	"22a_base",
	"22_exportfrom",
	"23_new",
	"24a_shared",
	"24b_middle",
	"24_mixed3a",
	"25_mathmacros",
	"26_stringmacros",
	"27_arraymacros",
	"28_collectionmacros",
}
