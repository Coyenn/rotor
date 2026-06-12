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
