package luau

import (
	"math"
	"testing"
)

// V8-verified vectors: each `want` is the exact output of String(number) in JS.
func TestJSNumberString(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{0, "0"},
		{math.Copysign(0, -1), "0"},
		{5, "5"},
		{-3, "-3"},
		{1.5, "1.5"},
		{0.1, "0.1"},
		{100, "100"},
		{1e20, "100000000000000000000"},
		{1e21, "1e+21"},
		{2e21, "2e+21"},
		{1.5e22, "1.5e+22"},
		{0.00001, "0.00001"},
		{0.000001, "0.000001"},
		{1e-7, "1e-7"},
		{1.234e-7, "1.234e-7"},
		{0.000015, "0.000015"},
		{9007199254740992, "9007199254740992"},
		{123.456, "123.456"},
		{math.Inf(1), "Infinity"},
		{math.Inf(-1), "-Infinity"},
		{math.NaN(), "NaN"},
	}
	for _, c := range cases {
		if got := JSNumberString(c.in); got != c.want {
			t.Errorf("JSNumberString(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestJSNumberParse(t *testing.T) {
	cases := []struct {
		in   string
		want float64
	}{
		{"5", 5},
		{"1_000", 1000},
		{"1.5", 1.5},
		{".5", 0.5},
		{"0xFF", 255},
		{"0b101", 5},
		{"0o17", 15},
		{"1e3", 1000},
	}
	for _, c := range cases {
		got, err := JSNumberParse(c.in)
		if err != nil || got != c.want {
			t.Errorf("JSNumberParse(%q) = %v, %v, want %v", c.in, got, err, c.want)
		}
	}
	if _, err := JSNumberParse("garbage"); err == nil {
		t.Error("JSNumberParse(\"garbage\") must return an error")
	}
}
