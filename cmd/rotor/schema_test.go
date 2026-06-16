package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"rotor/internal/config"
)

// `rotor schema` emits the canonical rotor.toml JSON Schema verbatim (so it can
// be redirected to publish the hosted file or to keep a local copy). The output
// must be byte-identical to config.Schema and itself valid JSON.
func TestCmdSchemaPrintsSchema(t *testing.T) {
	var buf bytes.Buffer
	if code := writeSchema(&buf); code != 0 {
		t.Fatalf("writeSchema exit = %d", code)
	}
	if buf.String() != config.Schema {
		t.Error("writeSchema output differs from config.Schema")
	}
	var v any
	if err := json.Unmarshal(buf.Bytes(), &v); err != nil {
		t.Fatalf("schema output is not valid JSON: %v", err)
	}
}

// An unknown argument is a usage error (exit 1); --help is exit 0.
func TestCmdSchemaArgs(t *testing.T) {
	if code := cmdSchema([]string{"--help"}); code != 0 {
		t.Errorf("--help exit = %d, want 0", code)
	}
	if code := cmdSchema([]string{"--bogus"}); code != 1 {
		t.Errorf("unknown arg exit = %d, want 1", code)
	}
}
