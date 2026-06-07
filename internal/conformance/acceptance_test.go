package conformance

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"rotor/internal/compile"
)

func resolveRandomnessProjectPath() (string, error) {
	raw := strings.TrimSpace(os.Getenv("ROTOR_RANDOMNESS_PATH"))
	if raw == "" {
		return "", fmt.Errorf("ROTOR_RANDOMNESS_PATH not set; point it at the randomness project root or its tsconfig.json")
	}
	info, err := os.Stat(raw)
	if err != nil {
		return "", fmt.Errorf("ROTOR_RANDOMNESS_PATH %q is not readable: %w", raw, err)
	}
	if !info.IsDir() {
		if filepath.Base(raw) != "tsconfig.json" {
			return "", fmt.Errorf("ROTOR_RANDOMNESS_PATH %q must be a project directory or tsconfig.json", raw)
		}
		raw = filepath.Dir(raw)
	}
	if _, err := os.Stat(filepath.Join(raw, "tsconfig.json")); err != nil {
		return "", fmt.Errorf("ROTOR_RANDOMNESS_PATH %q does not contain tsconfig.json: %w", raw, err)
	}
	return raw, nil
}

func TestResolveRandomnessProjectPath(t *testing.T) {
	t.Run("accepts project root", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Setenv("ROTOR_RANDOMNESS_PATH", dir)
		got, err := resolveRandomnessProjectPath()
		if err != nil {
			t.Fatal(err)
		}
		if got != dir {
			t.Fatalf("path = %q, want %q", got, dir)
		}
	})

	t.Run("accepts tsconfig path", func(t *testing.T) {
		dir := t.TempDir()
		tsconfig := filepath.Join(dir, "tsconfig.json")
		if err := os.WriteFile(tsconfig, []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Setenv("ROTOR_RANDOMNESS_PATH", tsconfig)
		got, err := resolveRandomnessProjectPath()
		if err != nil {
			t.Fatal(err)
		}
		if got != dir {
			t.Fatalf("path = %q, want %q", got, dir)
		}
	})
}

func TestRandomnessAcceptance(t *testing.T) {
	path, err := resolveRandomnessProjectPath()
	if err != nil {
		t.Skip(err.Error())
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
