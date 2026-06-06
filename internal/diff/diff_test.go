package diff

import (
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

func TestDifferential(t *testing.T) {
	root := repoRoot(t)
	projDir := filepath.Join(root, "testdata", "diff", "project")
	goldenDir := filepath.Join(root, "testdata", "diff", "golden")

	goldens, err := filepath.Glob(filepath.Join(goldenDir, "*.luau"))
	if err != nil || len(goldens) == 0 {
		t.Fatalf("no goldens found (%v) — run tools/oracle/oracle.ps1", err)
	}

	enabled := map[string]bool{}
	for _, name := range EnabledFixtures {
		enabled[name] = true
	}

	skipped := []string{}
	for _, goldenPath := range goldens {
		name := strings.TrimSuffix(filepath.Base(goldenPath), ".luau")
		if !enabled[name] {
			skipped = append(skipped, name)
			continue
		}
		t.Run(name, func(t *testing.T) {
			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatal(err)
			}
			got, diags, err := compile.CompileFile(projDir, filepath.Join("src", name+".ts"))
			if err != nil {
				t.Fatalf("compile error: %v", err)
			}
			if len(diags) > 0 {
				for _, d := range diags {
					t.Errorf("diagnostic: %s", d)
				}
				t.FailNow()
			}
			if got != string(want) {
				t.Errorf("output differs from rbxtsc golden")
				reportFirstDiff(t, got, string(want))
			}
		})
	}
	if len(skipped) > 0 {
		t.Logf("skipped (not yet enabled): %s", strings.Join(skipped, ", "))
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
