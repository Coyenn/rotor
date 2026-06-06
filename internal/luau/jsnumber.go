package luau

import (
	"math"
	"strconv"
	"strings"
)

// JSNumberString formats f exactly as ECMAScript Number::toString(10)
// (the JS String(number) operation), which roblox-ts relies on for emit.
// Port of ECMA-262 §6.1.6.1.20 Number::toString.
func JSNumberString(f float64) string {
	if math.IsNaN(f) {
		return "NaN"
	}
	if f == 0 {
		return "0" // covers -0 as well
	}
	if f < 0 {
		return "-" + JSNumberString(-f)
	}
	if math.IsInf(f, 1) {
		return "Infinity"
	}

	// Obtain the shortest round-trip digits and decimal exponent. The 'e'
	// format with -1 precision yields `d.dddde±XX`; s is the digit string
	// (k digits, leading digit first) and n satisfies f = 0.s × 10^n.
	repr := string(strconv.AppendFloat(nil, f, 'e', -1, 64))
	eIdx := strings.IndexByte(repr, 'e')
	s := strings.Replace(repr[:eIdx], ".", "", 1)
	exp, _ := strconv.Atoi(repr[eIdx+1:])
	k := len(s)
	n := exp + 1

	switch {
	case k <= n && n <= 21:
		return s + strings.Repeat("0", n-k)
	case 0 < n && n <= 21:
		return s[:n] + "." + s[n:]
	case -6 < n && n <= 0:
		return "0." + strings.Repeat("0", -n) + s
	}

	// Exponential notation. JS writes the exponent without zero padding,
	// e.g. "1e-7" and "1e+21".
	mantissa := s
	if k > 1 {
		mantissa = s[:1] + "." + s[1:]
	}
	e := n - 1
	sign := "+"
	if e < 0 {
		sign = "-"
		e = -e
	}
	return mantissa + "e" + sign + strconv.Itoa(e)
}

// JSNumberParse mirrors JS Number(text) for the literal forms the compiler
// emits: decimal floats (with optional `_` numeric separators) plus
// 0x/0b/0o integer forms that Go's ParseFloat rejects.
func JSNumberParse(text string) (float64, error) {
	cleaned := strings.ReplaceAll(text, "_", "")
	f, err := strconv.ParseFloat(cleaned, 64)
	if err == nil {
		return f, nil
	}
	if i, err2 := strconv.ParseInt(cleaned, 0, 64); err2 == nil {
		return float64(i), nil
	}
	return 0, err
}
