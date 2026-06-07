package conformance

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"rotor/internal/compile"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

// goldenPaths walks testdata/conformance/golden and returns every golden's
// path relative to the golden root, slash-separated (the manifest key form).
func goldenPaths(t *testing.T, goldenDir string) []string {
	t.Helper()
	var rels []string
	err := filepath.WalkDir(goldenDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".luau") {
			return nil
		}
		rel, err := filepath.Rel(goldenDir, path)
		if err != nil {
			return err
		}
		rels = append(rels, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatalf("walking goldens: %v", err)
	}
	return rels
}

func TestConformance(t *testing.T) {
	root := repoRoot(t)
	projDir := filepath.Join(root, "testdata", "conformance", "project")
	goldenDir := filepath.Join(root, "testdata", "conformance", "golden")

	goldens := goldenPaths(t, goldenDir)
	if len(goldens) == 0 {
		t.Fatal("no goldens found — run tools/oracle/conformance-oracle.ps1")
	}

	enabled := map[string]bool{}
	for _, name := range EnabledFixtures {
		enabled[name] = true
	}
	for name := range enabled {
		if _, err := os.Stat(filepath.Join(goldenDir, filepath.FromSlash(name))); err != nil {
			t.Errorf("manifest entry %q has no golden: %v", name, err)
		}
	}

	// Everything-disabled fast path: do NOT compile the project, so this test
	// stays green regardless of transformer state until Phase 5 enables
	// fixtures.
	if len(enabled) == 0 {
		t.Logf("conformance corpus disabled: all %d goldens skipped (manifest empty)", len(goldens))
		t.Skipf("no fixtures enabled in internal/conformance/manifest.go")
	}

	// ONE project-wide compile (the corpus shares a tsconfig and the specs
	// reference common helper modules); each enabled manifest entry then
	// diffs its out-file against the rbxtsc golden.
	out, diags, err := compile.CompileProject(projDir)
	if err != nil {
		t.Fatalf("CompileProject error: %v (diagnostics: %v)", err, diags)
	}
	if len(diags) > 0 {
		t.Fatalf("CompileProject diagnostics: %v", diags)
	}

	skipped := []string{}
	for _, rel := range goldens {
		if !enabled[rel] {
			skipped = append(skipped, rel)
			continue
		}
		t.Run(rel, func(t *testing.T) {
			want, err := os.ReadFile(filepath.Join(goldenDir, filepath.FromSlash(rel)))
			if err != nil {
				t.Fatal(err)
			}
			got, ok := out["out/"+rel]
			if !ok {
				t.Fatalf("out/%s missing from CompileProject output", rel)
			}
			if got != string(want) {
				t.Errorf("output differs from rbxtsc golden")
				reportFirstDiff(t, got, string(want))
			}
		})
	}
	if len(skipped) > 0 {
		t.Logf("skipped %d goldens (not yet enabled): %s", len(skipped), strings.Join(skipped, ", "))
	}
}

func reportFirstDiff(t *testing.T, got, want string) {
	t.Helper()
	gl, wl := strings.Split(got, "\n"), strings.Split(want, "\n")
	for i := 0; i < len(gl) && i < len(wl); i++ {
		if gl[i] != wl[i] {
			t.Errorf("first diff at line %d:\n  got:  %q\n  want: %q", i+1, gl[i], wl[i])
			return
		}
	}
	t.Errorf("length mismatch: got %d lines, want %d lines\n--- got tail ---\n%s\n--- want tail ---\n%s",
		len(gl), len(wl), tail(gl), tail(wl))
}

func tail(lines []string) string {
	n := len(lines)
	if n > 5 {
		lines = lines[n-5:]
	}
	return strings.Join(lines, "\n")
}
