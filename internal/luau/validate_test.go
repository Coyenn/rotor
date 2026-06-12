package luau

import (
	"regexp"
	"testing"
)

// The hand-rolled scanners in validate.go replaced these exact regexes;
// brute-force every short string over a covering alphabet to prove the
// behavior is byte-for-byte identical.
var (
	oracleIdentifierRegex  = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	oracleDecimalRegex     = regexp.MustCompile(`^(?:\d[\d_]*(?:\.[\d_]*)?|\.\d[\d_]*)(?:[eE][+-]?_*\d[\d_]*)?$`)
	oracleBinaryRegex      = regexp.MustCompile(`^0_*[bB]_*[01][01_]*$`)
	oracleHexadecimalRegex = regexp.MustCompile(`^0_*[xX]_*[\da-fA-F][\da-fA-F_]*$`)
)

func forEachShortString(alphabet string, maxLen int, fn func(string)) {
	var rec func(prefix string, depth int)
	rec = func(prefix string, depth int) {
		fn(prefix)
		if depth == maxLen {
			return
		}
		for i := 0; i < len(alphabet); i++ {
			rec(prefix+alphabet[i:i+1], depth+1)
		}
	}
	rec("", 0)
}

func TestIsValidIdentifierMatchesRegexOracle(t *testing.T) {
	oracle := func(s string) bool {
		if _, reserved := luauReservedKeywords[s]; reserved {
			return false
		}
		return oracleIdentifierRegex.MatchString(s)
	}
	forEachShortString("aZ0_ .\x80", 4, func(s string) {
		if got, want := IsValidIdentifier(s), oracle(s); got != want {
			t.Fatalf("IsValidIdentifier(%q) = %v, regex oracle says %v", s, got, want)
		}
	})
	for _, kw := range []string{"and", "do", "end", "nil", "while", "function"} {
		if IsValidIdentifier(kw) {
			t.Errorf("reserved keyword %q must be invalid", kw)
		}
	}
}

func TestIsValidNumberLiteralMatchesRegexOracle(t *testing.T) {
	oracle := func(s string) bool {
		return oracleDecimalRegex.MatchString(s) ||
			oracleBinaryRegex.MatchString(s) ||
			oracleHexadecimalRegex.MatchString(s)
	}
	// Alphabet covers every branch: leading zero, base prefixes (both
	// cases), binary/hex/decimal digit classes, underscores, dot,
	// exponent markers, signs, and a non-member byte.
	forEachShortString("01fbBxXeE_.+-", 4, func(s string) {
		if got, want := IsValidNumberLiteral(s), oracle(s); got != want {
			t.Fatalf("IsValidNumberLiteral(%q) = %v, regex oracle says %v", s, got, want)
		}
	})
	// Longer hand-picked cases beyond the brute-force length.
	longCases := []string{
		"0_1", "0__b1", "0b__10_1_", "0x_F_f0", "0X__", "0b2", "0xG",
		"1_000.5_5e+_1_0", "12.e5", ".5e_2", "1e+", "1e_", "0__x__abc_def",
		"0_e1", "0___", "123_456_789", ".0e-0_0", "0b1010e1", "0xFFp1",
	}
	for _, s := range longCases {
		if got, want := IsValidNumberLiteral(s), oracle(s); got != want {
			t.Errorf("IsValidNumberLiteral(%q) = %v, regex oracle says %v", s, got, want)
		}
	}
}

func TestIsValidIdentifier(t *testing.T) {
	valid := []string{"x", "_foo", "A1_b2", "_"}
	invalid := []string{"and", "end", "nil", "local", "until", "then", "elseif", "repeat", "1x", "a-b", "a b", "", "héllo"}
	for _, s := range valid {
		if !IsValidIdentifier(s) {
			t.Errorf("%q should be valid", s)
		}
	}
	for _, s := range invalid {
		if IsValidIdentifier(s) {
			t.Errorf("%q should be invalid", s)
		}
	}
}

func TestIsValidNumberLiteral(t *testing.T) {
	valid := []string{"1", "1_000", "1.5", ".5", "1e5", "1E+5", "1.2e-3", "0b1010", "0B_1010", "0xFF", "0X_ff_0"}
	invalid := []string{"", "abc", "0b", "0x", "1.2.3", "e5"}
	for _, s := range valid {
		if !IsValidNumberLiteral(s) {
			t.Errorf("%q should be valid", s)
		}
	}
	for _, s := range invalid {
		if IsValidNumberLiteral(s) {
			t.Errorf("%q should be invalid", s)
		}
	}
}

func TestMetamethodsAndReserved(t *testing.T) {
	if !IsMetamethod("__index") || IsMetamethod("__banana") {
		t.Error("IsMetamethod")
	}
	if !IsReservedClassField("new") || !IsReservedClassField("__index") || IsReservedClassField("foo") {
		t.Error("IsReservedClassField")
	}
	if !IsReservedIdentifier("TS") || !IsReservedIdentifier("game") || IsReservedIdentifier("myVar") {
		t.Error("IsReservedIdentifier")
	}
}
