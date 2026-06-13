package main

import (
	"path/filepath"
	"testing"
)

func TestResolveRojoProject(t *testing.T) {
	t.Run("explicit flag wins", func(t *testing.T) {
		dir := t.TempDir()
		if got := resolveRojoProject(dir, "custom.project.json"); got != "custom.project.json" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("default.project.json preferred", func(t *testing.T) {
		dir := t.TempDir()
		def := filepath.Join(dir, "default.project.json")
		other := filepath.Join(dir, "aaa.project.json")
		mustWrite(t, def, "{}")
		mustWrite(t, other, "{}")
		if got := resolveRojoProject(dir, ""); got != def {
			t.Fatalf("got %q, want %q", got, def)
		}
	})

	t.Run("falls back to any project.json", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "game.project.json")
		mustWrite(t, p, "{}")
		if got := resolveRojoProject(dir, ""); got != p {
			t.Fatalf("got %q, want %q", got, p)
		}
	})

	t.Run("none found", func(t *testing.T) {
		if got := resolveRojoProject(t.TempDir(), ""); got != "" {
			t.Fatalf("got %q, want empty", got)
		}
	})
}

