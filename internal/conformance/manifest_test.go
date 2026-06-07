package conformance

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFixtureManifestCoverage(t *testing.T) {
	root := repoRoot(t)
	goldenDir := filepath.Join(root, "testdata", "conformance", "golden")

	accounted := map[string]string{}
	for _, rel := range EnabledFixtures {
		if _, exists := accounted[rel]; exists {
			t.Fatalf("duplicate fixture entry for %q", rel)
		}
		accounted[rel] = "enabled"
	}
	for rel, reason := range DisabledFixtures {
		if reason == "" {
			t.Fatalf("disabled fixture %q missing reason", rel)
		}
		if _, exists := accounted[rel]; exists {
			t.Fatalf("fixture %q declared in both enabled and disabled manifests", rel)
		}
		accounted[rel] = "disabled"
	}

	for _, rel := range goldenPaths(t, goldenDir) {
		if _, err := os.Stat(filepath.Join(goldenDir, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("golden %q missing on disk: %v", rel, err)
		}
		if _, ok := accounted[rel]; !ok {
			t.Errorf("golden %q is neither enabled nor disabled", rel)
		}
	}
}
