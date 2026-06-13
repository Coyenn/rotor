package config

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// loadLegacyTS is the goja/esbuild path retained only for `rotor migrate`.
// These fixtures exercise the TypeScript/JavaScript evaluation pipeline.

func loadLegacy(t *testing.T, fixture string) *Config {
	t.Helper()
	cfg, err := LoadLegacyTS(filepath.Join("testdata", fixture))
	if err != nil {
		t.Fatalf("LoadLegacyTS(%q): %v", fixture, err)
	}
	return cfg
}

func TestLegacyLoadValidFullConfig(t *testing.T) {
	cfg := loadLegacy(t, "valid")
	if cfg.Assets == nil || cfg.Assets.Creator.Type != "group" || cfg.Assets.Creator.ID != 12345 {
		t.Fatalf("legacy assets = %+v", cfg.Assets)
	}
	if cfg.Deploy == nil || len(cfg.Deploy.Environments) != 2 {
		t.Fatalf("legacy deploy = %+v", cfg.Deploy)
	}
}

func TestLegacyLoadRelativeImport(t *testing.T) {
	cfg := loadLegacy(t, "relimport")
	if cfg.Assets == nil || cfg.Assets.Creator.Type != "user" || cfg.Assets.Creator.ID != 99 {
		t.Fatalf("creator from relative import = %+v", cfg.Assets)
	}
	if cfg.Deploy == nil || cfg.Deploy.Environments["dev"].UniverseID != 777 {
		t.Fatalf("universeId from imported function = %+v", cfg.Deploy)
	}
}

func TestLegacyLoadDirectModuleExports(t *testing.T) {
	cfg := loadLegacy(t, "jsdirect") // rotor.config.js, module.exports = {...}
	if cfg.Deploy == nil {
		t.Fatal("deploy section missing")
	}
	dev := cfg.Deploy.Environments["dev"]
	if dev.UniverseID != 42 || dev.Places["start"].PlaceID != 43 {
		t.Fatalf("jsdirect dev env = %+v", dev)
	}
}

func TestLegacyLoadMissingFile(t *testing.T) {
	cfg, err := LoadLegacyTS(t.TempDir())
	if cfg != nil {
		t.Fatalf("cfg = %+v, want nil", cfg)
	}
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestLegacyLoadNpmImport(t *testing.T) {
	_, err := LoadLegacyTS(filepath.Join("testdata", "npmimport"))
	if err == nil {
		t.Fatal("expected error for npm import")
	}
	if !strings.Contains(err.Error(), "npm imports") {
		t.Fatalf("err = %v, want mention of npm imports", err)
	}
	if !strings.Contains(err.Error(), `"zod"`) {
		t.Fatalf("err = %v, want offending module name", err)
	}
}

func TestLegacyLoadSyntaxError(t *testing.T) {
	_, err := LoadLegacyTS(filepath.Join("testdata", "syntaxerror"))
	if err == nil {
		t.Fatal("expected error for syntax error")
	}
	if !strings.Contains(err.Error(), "rotor.config.ts") {
		t.Fatalf("err = %v, want mention of rotor.config.ts", err)
	}
}

func TestLegacyLoadRuntimeError(t *testing.T) {
	_, err := LoadLegacyTS(filepath.Join("testdata", "runtimeerror"))
	if err == nil {
		t.Fatal("expected error for throwing config")
	}
	msg := err.Error()
	if !strings.Contains(msg, "boom from config") {
		t.Fatalf("err = %v, want the thrown message", err)
	}
	if !strings.Contains(msg, "rotor.config.ts") {
		t.Fatalf("err = %v, want mention of rotor.config.ts", err)
	}
}

func TestLegacyLoadUnknownKeyWarns(t *testing.T) {
	cfg := loadLegacy(t, "unknownkey")
	if len(cfg.Warnings) != 1 {
		t.Fatalf("warnings = %v, want exactly one", cfg.Warnings)
	}
	if !strings.Contains(cfg.Warnings[0], `"analytics"`) {
		t.Fatalf("warning = %q, want mention of the unknown key", cfg.Warnings[0])
	}
}
