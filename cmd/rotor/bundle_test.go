package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCmdBundleToFile(t *testing.T) {
	dir := t.TempDir()
	entry := filepath.Join(dir, "entry.luau")
	out := filepath.Join(dir, "out.luau")
	if err := os.WriteFile(entry, []byte("local m = require(\"./m\")\nreturn m\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "m.luau"), []byte("return 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := cmdBundle([]string{entry, "-o", out}); code != 0 {
		t.Fatalf("cmdBundle exit = %d", code)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if !strings.Contains(got, "__ROTOR_BUNDLE") || strings.Contains(got, "require(\"./m\")") {
		t.Fatalf("bundle output not as expected:\n%s", got)
	}
}

func TestCmdBundleMissingEntry(t *testing.T) {
	if code := cmdBundle(nil); code != 1 {
		t.Fatalf("expected exit 1 for missing entry, got %d", code)
	}
}

func TestCmdBundleBadEntry(t *testing.T) {
	if code := cmdBundle([]string{filepath.Join(t.TempDir(), "nope.luau")}); code != 1 {
		t.Fatalf("expected exit 1 for missing file, got %d", code)
	}
}

func TestCmdBundleParseErrorCodeFrame(t *testing.T) {
	dir := t.TempDir()
	entry := filepath.Join(dir, "entry.luau")
	// "return 1 2" — adjacent literals are a Luau parse error.
	src := "local x = 1\nreturn 1 2\n"
	if err := os.WriteFile(entry, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	stderr, code := captureStderr(t, func() int {
		return cmdBundle([]string{entry})
	})
	if code != 1 {
		t.Fatalf("expected exit 1 for parse error, got %d", code)
	}
	// The code frame must contain the offending source line.
	if !strings.Contains(stderr, "return 1 2") {
		t.Errorf("stderr does not contain offending source line %q\nstderr:\n%s", "return 1 2", stderr)
	}
	// The code frame must contain a caret pointing at the error.
	if !strings.Contains(stderr, "^") {
		t.Errorf("stderr does not contain a caret '^'\nstderr:\n%s", stderr)
	}
}
