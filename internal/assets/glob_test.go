package assets

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMatch(t *testing.T) {
	cases := []struct {
		pattern, name string
		want          bool
	}{
		// literal
		{"assets/logo.png", "assets/logo.png", true},
		{"assets/logo.png", "assets/logo.jpg", false},
		// * stays within a segment
		{"assets/*.png", "assets/logo.png", true},
		{"assets/*.png", "assets/sub/logo.png", false},
		{"assets/*.png", "logo.png", false},
		// ? is one character
		{"assets/v?.png", "assets/v1.png", true},
		{"assets/v?.png", "assets/v12.png", false},
		// ** spans segments, including zero
		{"assets/**/*.png", "assets/logo.png", true},
		{"assets/**/*.png", "assets/a/b/c/logo.png", true},
		{"assets/**/*.png", "assets/a/b/c/logo.ogg", false},
		{"assets/**/*.png", "other/logo.png", false},
		{"**/*.ogg", "sounds/hit.ogg", true},
		{"**/*.ogg", "hit.ogg", true},
		{"assets/**", "assets/deep/down/file.bin", true},
		{"assets/**", "elsewhere/file.bin", false},
		// case-insensitive (Windows-friendly)
		{"assets/**/*.png", "Assets/Logo.PNG", true},
		{"ASSETS/*.ogg", "assets/hit.ogg", true},
		// windows separators on either side
		{"assets\\*.png", "assets/logo.png", true},
		{"assets/*.png", "assets\\logo.png", true},
		{"assets\\**\\*.ogg", "assets\\sounds\\hit.ogg", true},
		// ./ prefixes and doubled slashes normalize away
		{"./assets/*.png", "assets/logo.png", true},
		{"assets//*.png", "assets/logo.png", true},
		// star backtracking inside a segment
		{"assets/*o*.png", "assets/logo.png", true},
		{"assets/*z*.png", "assets/logo.png", false},
	}
	for _, c := range cases {
		if got := Match(c.pattern, c.name); got != c.want {
			t.Errorf("Match(%q, %q) = %v, want %v", c.pattern, c.name, got, c.want)
		}
	}
}

func TestStaticPrefix(t *testing.T) {
	cases := []struct{ pattern, want string }{
		{"assets/**/*.png", "assets"},
		{"assets/img/*.png", "assets/img"},
		{"assets/logo.png", "assets/logo.png"},
		{"**/*.png", ""},
		{"./assets/*.ogg", "assets"},
	}
	for _, c := range cases {
		if got := staticPrefix(c.pattern); got != c.want {
			t.Errorf("staticPrefix(%q) = %q, want %q", c.pattern, got, c.want)
		}
	}
}

func TestScanClassifiesHashesAndSkips(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, content string) {
		t.Helper()
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("assets/logo.png", "png-bytes")
	write("assets/sounds/hit.ogg", "ogg-bytes")
	write("assets/notes.txt", "not an asset")
	write("assets/UPPER.PNG", "upper")
	write("src/code.luau", "return 1") // outside the globs

	res, err := Scan(dir, []string{"assets/**/*.png", "assets/**/*.ogg", "assets/**/*.txt", "assets/**/*.png"})
	if err != nil {
		t.Fatal(err)
	}

	var paths []string
	for _, f := range res.Files {
		paths = append(paths, f.Path)
	}
	want := []string{"assets/UPPER.PNG", "assets/logo.png", "assets/sounds/hit.ogg"}
	if strings.Join(paths, ",") != strings.Join(want, ",") {
		t.Fatalf("scanned files = %v, want %v", paths, want)
	}
	if res.Files[1].Type != TypeDecal || res.Files[2].Type != TypeAudio {
		t.Fatalf("bad classification: %+v", res.Files)
	}
	for _, f := range res.Files {
		if !strings.HasPrefix(f.Hash, "sha256:") || len(f.Hash) != len("sha256:")+64 {
			t.Fatalf("hash format wrong for %s: %q", f.Path, f.Hash)
		}
	}
	if len(res.Skipped) != 1 || res.Skipped[0] != "assets/notes.txt" {
		t.Fatalf("skipped = %v, want [assets/notes.txt]", res.Skipped)
	}

	// unmatched pattern is not an error
	res2, err := Scan(dir, []string{"missing/**/*.png"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res2.Files) != 0 {
		t.Fatalf("expected no files, got %v", res2.Files)
	}

	// identical content hashes identically; different content differs
	write("assets/logo2.png", "png-bytes")
	res3, err := Scan(dir, []string{"assets/logo.png", "assets/logo2.png", "assets/UPPER.PNG"})
	if err != nil {
		t.Fatal(err)
	}
	if res3.Files[1].Hash != res3.Files[2].Hash { // logo.png vs logo2.png
		t.Fatal("same content should hash the same")
	}
	if res3.Files[0].Hash == res3.Files[1].Hash { // UPPER.PNG vs logo.png
		t.Fatal("different content should hash differently")
	}
}
