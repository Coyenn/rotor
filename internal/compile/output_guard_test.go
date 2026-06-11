package compile

import (
	"runtime"
	"testing"
)

func TestAssertLocalOutputPath(t *testing.T) {
	for _, ok := range []string{
		"out/main.luau",
		"out/sub/init.luau",
		"out/weird..name.luau",
	} {
		if err := assertLocalOutputPath(ok); err != nil {
			t.Errorf("assertLocalOutputPath(%q) = %v, want nil", ok, err)
		}
	}
	// Note: "C:/x" is only non-local on Windows, so it is asserted in the
	// Windows-tagged half below rather than here.
	for _, bad := range []string{
		"../escape.luau",
		"out/../../escape.luau",
		"/abs/escape.luau",
		"..",
	} {
		if err := assertLocalOutputPath(bad); err == nil {
			t.Errorf("assertLocalOutputPath(%q) = nil, want error", bad)
		}
	}
	if runtime.GOOS == "windows" {
		if err := assertLocalOutputPath("C:/abs/escape.luau"); err == nil {
			t.Error(`assertLocalOutputPath("C:/abs/escape.luau") = nil, want error on Windows`)
		}
	}
}
