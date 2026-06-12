package dotenv

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseBasics(t *testing.T) {
	vars := Parse("KEY=value\nOTHER=two words\nNUM=42\n")
	want := map[string]string{"KEY": "value", "OTHER": "two words", "NUM": "42"}
	for k, v := range want {
		if vars[k] != v {
			t.Errorf("vars[%q] = %q, want %q", k, vars[k], v)
		}
	}
	if len(vars) != len(want) {
		t.Errorf("len(vars) = %d, want %d (%v)", len(vars), len(want), vars)
	}
}

func TestParseCRLF(t *testing.T) {
	// Windows-authored .env files end lines with \r\n; the \r must not leak
	// into values.
	vars := Parse("A=one\r\nB=two\r\n")
	if vars["A"] != "one" || vars["B"] != "two" {
		t.Errorf("CRLF parse: %v", vars)
	}
}

func TestParseCommentsAndBlanks(t *testing.T) {
	vars := Parse("# full line comment\n\n   \n  # indented comment\nKEY=value\n")
	if len(vars) != 1 || vars["KEY"] != "value" {
		t.Errorf("vars = %v, want only KEY=value", vars)
	}
}

func TestParseQuotes(t *testing.T) {
	vars := Parse(
		"D=\"double quoted\"\n" +
			"S='single quoted'\n" +
			"INNER=\"  keeps inner spaces  \"\n" +
			"MIXED=\"unbalanced'\n" +
			"EMPTYQ=\"\"\n" +
			"HASH=\"# not a comment\"\n")
	cases := map[string]string{
		"D":      "double quoted",
		"S":      "single quoted",
		"INNER":  "  keeps inner spaces  ",
		"MIXED":  "\"unbalanced'", // mismatched quotes are kept verbatim
		"EMPTYQ": "",
		"HASH":   "# not a comment",
	}
	for k, want := range cases {
		if got, ok := vars[k]; !ok || got != want {
			t.Errorf("vars[%q] = %q (ok=%v), want %q", k, got, ok, want)
		}
	}
}

func TestParseWhitespaceAndJunk(t *testing.T) {
	vars := Parse("  PADDED  =  spaced out  \nNOEQUALS\n=novalue\nEMPTY=\nLAST=wins\nLAST=really wins\n")
	if vars["PADDED"] != "spaced out" {
		t.Errorf("PADDED = %q", vars["PADDED"])
	}
	if _, ok := vars["NOEQUALS"]; ok {
		t.Error("line without = should be ignored")
	}
	if _, ok := vars[""]; ok {
		t.Error("empty key should be ignored")
	}
	if v, ok := vars["EMPTY"]; !ok || v != "" {
		t.Errorf("EMPTY = %q (ok=%v), want empty string present", v, ok)
	}
	if vars["LAST"] != "really wins" {
		t.Errorf("LAST = %q, want later duplicate to win", vars["LAST"])
	}
}

func writeFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadPrecedence(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".env"),
		"BASE=base\r\nOVERLAY=from-base\r\nPROC=from-file\r\nNODE_ENV=studio\r\n")
	writeFile(t, filepath.Join(dir, ".env.studio"),
		"OVERLAY=from-studio\n")

	// NODE_ENV comes from the .env file here (unset in the process env).
	t.Setenv("NODE_ENV", "")
	t.Setenv("PROC", "from-process")

	env := Load(dir)

	cases := []struct {
		name  string
		want  string
		found bool
	}{
		{"BASE", "base", true},               // .env only
		{"OVERLAY", "from-studio", true},     // .env.<NODE_ENV> beats .env
		{"PROC", "from-process", true},       // process env beats files
		{"ROTOR_DOTENV_UNSET_42", "", false}, // defined nowhere
		{"NODE_ENV", "", true},               // process env ("" set above) wins
	}
	for _, c := range cases {
		got, ok := env.Lookup(c.name)
		if ok != c.found || got != c.want {
			t.Errorf("Lookup(%q) = (%q, %v), want (%q, %v)", c.name, got, ok, c.want, c.found)
		}
	}
}

func TestLoadNodeEnvFromProcess(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".env"), "NODE_ENV=studio\nWHO=base\n")
	writeFile(t, filepath.Join(dir, ".env.studio"), "WHO=studio\n")
	writeFile(t, filepath.Join(dir, ".env.production"), "WHO=production\n")

	// Process NODE_ENV outranks the .env-declared one.
	t.Setenv("NODE_ENV", "production")
	env := Load(dir)
	if got, _ := env.Lookup("WHO"); got != "production" {
		t.Errorf("WHO = %q, want %q", got, "production")
	}
}

func TestLoadMissingFiles(t *testing.T) {
	env := Load(t.TempDir())
	if _, ok := env.Lookup("ROTOR_DOTENV_UNSET_42"); ok {
		t.Error("unset name resolved with no .env files present")
	}
}

func TestNilEnvLookup(t *testing.T) {
	var env *Env
	t.Setenv("ROTOR_DOTENV_NIL_RECEIVER", "ok")
	if got, ok := env.Lookup("ROTOR_DOTENV_NIL_RECEIVER"); !ok || got != "ok" {
		t.Errorf("nil receiver Lookup = (%q, %v), want process env hit", got, ok)
	}
	if _, ok := env.Lookup("ROTOR_DOTENV_UNSET_42"); ok {
		t.Error("nil receiver resolved an unset name")
	}
}
