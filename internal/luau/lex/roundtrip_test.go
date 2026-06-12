package lex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Concatenating every token's Text must reproduce the source exactly.
func assertRoundtrip(t *testing.T, name, src string) {
	t.Helper()
	toks, _ := Tokenize(src)
	var b strings.Builder
	for _, tk := range toks {
		b.WriteString(tk.Text)
	}
	if b.String() != src {
		t.Fatalf("roundtrip mismatch for %s (len %d vs %d)", name, b.Len(), len(src))
	}
}

func TestRoundtripUnits(t *testing.T) {
	cases := []string{
		"",
		"\n\n",
		"local x = 1 -- comment\n",
		"return `a{b}c`\n",
		"local s = [==[ raw ]] still ]==]\n",
		"print('hi')\r\nprint(\"there\")",
		"local t = { a = 1, [\"b\"] = 2, 3 }",
		"x += 1 // 2 .. 'z'",
		"--no trailing newline",
		"`{ {nested=`{inner}`} }`",
	}
	for i, src := range cases {
		assertRoundtrip(t, "unit"+string(rune('0'+i)), src)
	}
}

// TestRoundtripCorpus walks real .luau/.lua files if a corpus root is provided.
// Run with: ROTOR_LUAU_CORPUS=<dir> go test ./internal/luau/lex/ -run TestRoundtripCorpus
func TestRoundtripCorpus(t *testing.T) {
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
		assertRoundtrip(t, path, string(data))
		count++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("roundtripped %d files", count)
}
