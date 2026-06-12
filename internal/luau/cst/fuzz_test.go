package cst

import "testing"

func FuzzParse(f *testing.F) {
	seeds := []string{
		"local x = 1", "return a, b", "if x then y() end",
		"for i = 1, 10 do end", "for k, v in pairs(t) do end",
		"function f(...) return ... end", "local t: {number} = {}",
		"x += 1", "a `b{c}d`", "({1})[2] = 3", "type X<T...> = (T...) -> ()",
		"local f: <T>(T) -> T? = nil", "repeat until true", "while a do break end",
		"do continue end", "local a, b = 1, 2", "export type Y = { x: number }",
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, src string) {
		file, diags := Parse(src) // must never panic or hang
		if len(diags) == 0 {
			if Unparse(file) != src {
				t.Fatalf("clean parse must roundtrip: %q", src)
			}
		}
	})
}
