package conformance

import (
	"os"
	"path/filepath"
	"testing"

	"rotor/internal/compile"
)

func TestRandomnessAcceptance(t *testing.T) {
	path := os.Getenv("ROTOR_RANDOMNESS_PATH")
	if path == "" {
		t.Skip("ROTOR_RANDOMNESS_PATH not set")
	}
	if _, err := os.Stat(filepath.Join(path, "tsconfig.json")); err != nil {
		t.Fatalf("randomness tsconfig missing: %v", err)
	}

	out, diags, err := compile.CompileProject(path)
	if err != nil {
		t.Fatalf("CompileProject: %v (diags: %v)", err, diags)
	}
	if len(diags) > 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if len(out) == 0 {
		t.Fatal("no emitted files")
	}
}
