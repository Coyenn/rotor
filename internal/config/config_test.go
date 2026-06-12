package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func load(t *testing.T, fixture string) *Config {
	t.Helper()
	cfg, err := Load(filepath.Join("testdata", fixture))
	if err != nil {
		t.Fatalf("Load(%q): %v", fixture, err)
	}
	return cfg
}

func TestLoadValidFullConfig(t *testing.T) {
	cfg := load(t, "valid")

	if cfg.Assets == nil {
		t.Fatal("assets section missing")
	}
	wantPaths := []string{"assets/**/*.png", "assets/**/*.ogg"}
	if len(cfg.Assets.Paths) != len(wantPaths) {
		t.Fatalf("assets.paths = %v, want %v", cfg.Assets.Paths, wantPaths)
	}
	for i, p := range wantPaths {
		if cfg.Assets.Paths[i] != p {
			t.Errorf("assets.paths[%d] = %q, want %q", i, cfg.Assets.Paths[i], p)
		}
	}
	if cfg.Assets.Output.Luau != "src/shared/assets.luau" {
		t.Errorf("assets.output.luau = %q", cfg.Assets.Output.Luau)
	}
	if cfg.Assets.Output.Types != "src/shared/assets.d.ts" {
		t.Errorf("assets.output.types = %q", cfg.Assets.Output.Types)
	}
	if cfg.Assets.Creator.Type != "group" || cfg.Assets.Creator.ID != 12345 {
		t.Errorf("assets.creator = %+v", cfg.Assets.Creator)
	}

	if cfg.Deploy == nil {
		t.Fatal("deploy section missing")
	}
	if len(cfg.Deploy.Environments) != 2 {
		t.Fatalf("environments = %v", cfg.Deploy.Environments)
	}

	dev := cfg.Deploy.Environments["dev"]
	if dev.UniverseID != 111 {
		t.Errorf("dev.universeId = %d", dev.UniverseID)
	}
	if dev.Payments != "free" {
		t.Errorf("dev.payments = %q", dev.Payments)
	}
	if p := dev.Places["start"]; p.File != "build/game.rbxl" || p.PlaceID != 222 {
		t.Errorf("dev.places.start = %+v", p)
	}
	if dev.Experience != nil {
		t.Errorf("dev.experience = %+v, want nil", dev.Experience)
	}

	prod := cfg.Deploy.Environments["prod"]
	if prod.UniverseID != 333 {
		t.Errorf("prod.universeId = %d", prod.UniverseID)
	}
	if len(prod.Places) != 2 {
		t.Errorf("prod.places = %+v", prod.Places)
	}
	if p := prod.Places["lobby"]; p.File != "build/lobby.rbxl" || p.PlaceID != 555 {
		t.Errorf("prod.places.lobby = %+v", p)
	}
	if prod.Experience == nil {
		t.Fatal("prod.experience missing")
	}
	if prod.Experience.Name != "My Game" ||
		prod.Experience.Description != "The best game" ||
		prod.Experience.Playability != "public" {
		t.Errorf("prod.experience = %+v", prod.Experience)
	}
	if prod.Payments != "paid" {
		t.Errorf("prod.payments = %q", prod.Payments)
	}
	b := prod.Badges["winner"]
	if b.Name != "Winner!" || b.Description != "You won" || b.Icon != "assets/badge.png" {
		t.Errorf("prod.badges.winner = %+v", b)
	}

	if len(cfg.Warnings) != 0 {
		t.Errorf("unexpected warnings: %v", cfg.Warnings)
	}
	if errs := cfg.Validate(); len(errs) != 0 {
		t.Errorf("Validate() = %v, want clean", errs)
	}
}

func TestLoadRelativeImport(t *testing.T) {
	cfg := load(t, "relimport")
	if cfg.Assets == nil || cfg.Assets.Creator.Type != "user" || cfg.Assets.Creator.ID != 99 {
		t.Fatalf("creator from relative import = %+v", cfg.Assets)
	}
	if cfg.Deploy == nil || cfg.Deploy.Environments["dev"].UniverseID != 777 {
		t.Fatalf("universeId from imported function = %+v", cfg.Deploy)
	}
}

func TestLoadDirectModuleExports(t *testing.T) {
	cfg := load(t, "jsdirect") // rotor.config.js, module.exports = {...}
	if cfg.Deploy == nil {
		t.Fatal("deploy section missing")
	}
	dev := cfg.Deploy.Environments["dev"]
	if dev.UniverseID != 42 || dev.Places["start"].PlaceID != 43 {
		t.Fatalf("jsdirect dev env = %+v", dev)
	}
}

func TestLoadMissingFile(t *testing.T) {
	cfg, err := Load(t.TempDir())
	if cfg != nil {
		t.Fatalf("cfg = %+v, want nil", cfg)
	}
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestLoadNpmImport(t *testing.T) {
	_, err := Load(filepath.Join("testdata", "npmimport"))
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

func TestLoadSyntaxError(t *testing.T) {
	_, err := Load(filepath.Join("testdata", "syntaxerror"))
	if err == nil {
		t.Fatal("expected error for syntax error")
	}
	if !strings.Contains(err.Error(), "rotor.config.ts") {
		t.Fatalf("err = %v, want mention of rotor.config.ts", err)
	}
}

func TestLoadRuntimeError(t *testing.T) {
	_, err := Load(filepath.Join("testdata", "runtimeerror"))
	if err == nil {
		t.Fatal("expected error for throwing config")
	}
	msg := err.Error()
	if !strings.Contains(msg, "boom from config") {
		t.Fatalf("err = %v, want the thrown message", err)
	}
	// Positions are sourcemapped back to the original file via the inline
	// sourcemap; at minimum the original file name must appear.
	if !strings.Contains(msg, "rotor.config.ts") {
		t.Fatalf("err = %v, want mention of rotor.config.ts", err)
	}
}

func TestLoadUnknownKeyWarns(t *testing.T) {
	cfg := load(t, "unknownkey")
	if len(cfg.Warnings) != 1 {
		t.Fatalf("warnings = %v, want exactly one", cfg.Warnings)
	}
	if !strings.Contains(cfg.Warnings[0], `"analytics"`) {
		t.Fatalf("warning = %q, want mention of the unknown key", cfg.Warnings[0])
	}
	if cfg.Assets == nil || cfg.Assets.Creator.ID != 1 {
		t.Fatalf("known sections must still load: %+v", cfg.Assets)
	}
}

func TestValidate(t *testing.T) {
	t.Run("bad creator type", func(t *testing.T) {
		cfg := &Config{Assets: &AssetsConfig{Creator: Creator{Type: "person", ID: 1}}}
		errs := cfg.Validate()
		if len(errs) != 1 || !strings.Contains(errs[0].Error(), "creator.type") {
			t.Fatalf("Validate() = %v, want one creator.type error", errs)
		}
	})

	t.Run("place missing file and placeId", func(t *testing.T) {
		cfg := &Config{Deploy: &DeployConfig{Environments: map[string]Environment{
			"dev": {UniverseID: 1, Places: map[string]PlaceDeploy{"start": {}}},
		}}}
		errs := cfg.Validate()
		if len(errs) != 2 {
			t.Fatalf("Validate() = %v, want file + placeId errors", errs)
		}
	})

	t.Run("bad playability", func(t *testing.T) {
		cfg := &Config{Deploy: &DeployConfig{Environments: map[string]Environment{
			"prod": {UniverseID: 1, Experience: &ExperienceConfig{Playability: "everyone"}},
		}}}
		errs := cfg.Validate()
		if len(errs) != 1 || !strings.Contains(errs[0].Error(), "playability") {
			t.Fatalf("Validate() = %v, want one playability error", errs)
		}
	})

	t.Run("empty config is valid", func(t *testing.T) {
		if errs := (&Config{}).Validate(); len(errs) != 0 {
			t.Fatalf("Validate() = %v, want clean", errs)
		}
	})
}

func TestTypeDeclarations(t *testing.T) {
	if !strings.Contains(TypeDeclarations, `declare module "@rotor-rbx/rotor"`) {
		t.Fatal("missing declare module")
	}
	for _, name := range []string{
		"AssetsOutput", "Creator", "AssetsConfig", "PlaceDeploy",
		"ExperienceConfig", "Badge", "Environment", "DeployConfig", "Config",
	} {
		if !strings.Contains(TypeDeclarations, "export interface "+name+" {") {
			t.Errorf("missing interface %s", name)
		}
	}
	if !strings.Contains(TypeDeclarations, "export function defineConfig(config: Config): Config;") {
		t.Error("missing defineConfig declaration")
	}
	// A stray "*/" inside a doc comment (e.g. from a glob example) would
	// truncate the declarations; every /** must pair with exactly one */.
	if open, closed := strings.Count(TypeDeclarations, "/*"), strings.Count(TypeDeclarations, "*/"); open != closed {
		t.Errorf("unbalanced block comments: %d open, %d close", open, closed)
	}
}

func TestWriteTypeDeclarations(t *testing.T) {
	dir := t.TempDir()
	if err := WriteTypeDeclarations(dir); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, TypeDeclarationsFileName))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != TypeDeclarations {
		t.Fatal("written file does not match TypeDeclarations")
	}
}
