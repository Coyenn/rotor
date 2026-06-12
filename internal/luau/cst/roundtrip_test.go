package cst

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// AttachTrivia + Flatten must reproduce any source exactly.
func TestTriviaRoundtripCorpus(t *testing.T) {
	root := os.Getenv("ROTOR_LUAU_CORPUS")
	if root == "" {
		t.Skip("set ROTOR_LUAU_CORPUS to a directory of .luau/.lua files")
	}
	count := 0
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if !strings.HasSuffix(path, ".luau") && !strings.HasSuffix(path, ".lua") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		src := string(data)
		if got := Flatten(AttachTrivia(src)); got != src {
			t.Fatalf("trivia roundtrip mismatch: %s", path)
		}
		count++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("trivia-roundtripped %d files", count)
}

// TestParseRoundtripCorpus is the real parser gate: every corpus file must parse
// with zero diagnostics and Unparse back to the exact source.
func TestParseRoundtripCorpus(t *testing.T) {
	root := os.Getenv("ROTOR_LUAU_CORPUS")
	if root == "" {
		t.Skip("set ROTOR_LUAU_CORPUS to a directory of .luau/.lua files")
	}
	count, parsed := 0, 0
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if !strings.HasSuffix(path, ".luau") && !strings.HasSuffix(path, ".lua") {
			return nil
		}
		count++
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			t.Errorf("%s: %v", path, rerr)
			return nil
		}
		src := string(data)
		file, diags := Parse(src)
		if len(diags) != 0 {
			t.Errorf("%s: %d diag(s), first: %q at %v", path, len(diags), diags[0].Message, diags[0].Pos)
			return nil
		}
		if got := Unparse(file); got != src {
			t.Errorf("%s: parse roundtrip mismatch", path)
			return nil
		}
		parsed++
		return nil
	})
	t.Logf("parsed+roundtripped %d/%d files", parsed, count)
}
