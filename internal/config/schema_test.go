package config

import (
	"os"
	"path/filepath"
	"testing"
)

// writeConfigFile writes content to dir/name, creating parent dirs.
func writeConfigFile(t *testing.T, dir, name, content string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The schema is hosted (served from SchemaURL via raw GitHub) rather than
// written per-project, so a single rotor.schema.json is committed at the repo
// root. It must never drift from config.Schema — `rotor schema` regenerates it.
func TestCommittedSchemaMatchesConstant(t *testing.T) {
	const rel = "../../rotor.schema.json" // repo root, relative to internal/config
	data, err := os.ReadFile(rel)
	if err != nil {
		t.Fatalf("committed schema missing — run `rotor schema > rotor.schema.json` at the repo root: %v", err)
	}
	if string(data) != Schema {
		t.Errorf("%s is out of sync with config.Schema; regenerate with `rotor schema > rotor.schema.json`", rel)
	}
}
