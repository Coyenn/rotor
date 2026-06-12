package cst

import "testing"

func TestCursorBasics(t *testing.T) {
	c := newCursor("a + b")
	if c.peek().Token.Text != "a" {
		t.Fatalf("peek = %q", c.peek().Token.Text)
	}
	a := c.next()
	if a.Token.Text != "a" || c.peek().Token.Text != "+" {
		t.Fatalf("after next: %q peek %q", a.Token.Text, c.peek().Token.Text)
	}
	if !c.atSymbol("+") {
		t.Fatalf("expected + symbol")
	}
	if c.atEnd() {
		t.Fatalf("not at end yet")
	}
}

func roundtripFile(t *testing.T, src string) {
	t.Helper()
	file, diags := Parse(src)
	if len(diags) != 0 {
		t.Fatalf("%q: diags %v", src, diags)
	}
	if got := Unparse(file); got != src {
		t.Fatalf("file roundtrip: %q -> %q", src, got)
	}
}

func TestStatementRoundtrip(t *testing.T) {
	cases := []string{
		"",
		"\n",
		"-- just a comment\n",
		"local x = 1\n",
		"local a, b, c = 1, 2, 3\n",
		"local x: number = 1\n",
		"local t: { [string]: number } = {}\n",
		"local n <const> = 5\n",
		"x = 1\n",
		"x, y = y, x\n",
		"x += 1\n",
		"t.a.b = c\n",
		"t[k] = v\n",
		"print(\"hi\")\n",
		"foo:bar(1, 2)\n",
		"do\n\tlocal x = 1\nend\n",
		"while a do\n\tb()\nend\n",
		"repeat\n\tx()\nuntil done\n",
		"for i = 1, 10 do\n\tprint(i)\nend\n",
		"for i = 10, 1, -1 do end\n",
		"for k, v in pairs(t) do\n\tprint(k, v)\nend\n",
		"if a then\n\tb()\nelseif c then\n\td()\nelse\n\te()\nend\n",
		"function M.foo:bar(a, b)\n\treturn a + b\nend\n",
		"local function f(x)\n\treturn x\nend\n",
		"function f<T>(x: T): T\n\treturn x\nend\n",
		"function f(a, b, ...)\n\treturn ...\nend\n",
		"return\n",
		"return a, b\n",
		"return;\n",
		"do break end\n",
		"do continue end\n",
		"type Maybe<T> = T | nil\n",
		"export type Handler = (msg: string) -> ()\n",
		"type Pack<T...> = (T...) -> T...\n",
		"local f = function(x) return x * 2 end\n",
		"local x = a :: number\n",
		"local s = `hello {name}, you are {age} years old`\n",
		";\n",
		"local x = 1; local y = 2\n",
	}
	for _, src := range cases {
		roundtripFile(t, src)
	}
}
