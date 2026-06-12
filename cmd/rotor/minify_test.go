package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCmdMinifyToFile(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "in.luau")
	out := filepath.Join(dir, "out.luau")
	src := "--!strict\n-- drop me\nlocal   x   =   1\nreturn x\n"
	if err := os.WriteFile(in, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := cmdMinify([]string{in, "-o", out}); code != 0 {
		t.Fatalf("cmdMinify exit code = %d", code)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	want := "--!strict\nlocal x=1return x"
	if string(got) != want {
		t.Fatalf("minified = %q, want %q", got, want)
	}
}

func TestCmdMinifyMissingInput(t *testing.T) {
	if code := cmdMinify(nil); code != 1 {
		t.Fatalf("expected exit 1 for missing input, got %d", code)
	}
}

func TestCmdMinifyParseError(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "bad.luau")
	// An unterminated string is a lexer diagnostic; minify must fail (exit 1).
	if err := os.WriteFile(in, []byte("local x = \"oops\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := cmdMinify([]string{in}); code != 1 {
		t.Fatalf("expected exit 1 for parse error, got %d", code)
	}
}
