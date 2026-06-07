package conformance

import (
	"path/filepath"
	"testing"
)

func TestDiagnosticsCorpus(t *testing.T) {
	root := repoRoot(t)
	dir := filepath.Join(root, "testdata", "conformance", "excluded", "diagnostics")

	cases, err := loadDiagnosticFixtures(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) == 0 {
		t.Fatal("no diagnostic fixtures found")
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			if reason := skipReasonForDiagnosticFixture(tc); reason != "" {
				t.Skip(reason)
			}
			got, err := compileDiagnosticFixture(root, tc.Path)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) == 0 {
				t.Fatalf("expected diagnostic %q, got none", tc.ExpectedID)
			}
			for _, id := range got {
				if id != tc.ExpectedID {
					t.Fatalf("expected only %q, got %v", tc.ExpectedID, got)
				}
			}
		})
	}
}
