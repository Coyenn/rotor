package diagframe

import (
	"testing"

	"rotor/internal/luau"
)

func TestLuauKeywordSetMatchesCanonical(t *testing.T) {
	// Every diagframe Luau keyword must be canonical.
	for w := range luauKeywords {
		if !luau.IsReservedKeyword(w) {
			t.Errorf("diagframe lists %q but internal/luau does not", w)
		}
	}
	// Every canonical Luau keyword must be in diagframe's set.
	for _, w := range []string{
		"and", "break", "do", "else", "elseif", "end", "false", "for",
		"function", "if", "in", "local", "nil", "not", "or", "repeat",
		"return", "then", "true", "until", "while",
	} {
		if _, ok := luauKeywords[w]; !ok {
			t.Errorf("canonical keyword %q missing from diagframe", w)
		}
		if !luau.IsReservedKeyword(w) {
			t.Errorf("test list has %q but internal/luau rejects it", w)
		}
	}
	if len(luauKeywords) != 21 {
		t.Errorf("expected 21 Luau keywords, got %d", len(luauKeywords))
	}
}
