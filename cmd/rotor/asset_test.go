package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCmdAssetArgValidation(t *testing.T) {
	// no subcommand
	if code := cmdAsset(nil); code != 1 {
		t.Fatalf("no subcommand: exit %d", code)
	}
	// unknown subcommand
	if code := cmdAsset([]string{"upload"}); code != 1 {
		t.Fatalf("unknown subcommand: exit %d", code)
	}
	// unknown flag
	if code := cmdAsset([]string{"sync", "--bogus"}); code != 1 {
		t.Fatalf("unknown flag: exit %d", code)
	}
	// extra positional
	if code := cmdAsset([]string{"sync", "a", "b"}); code != 1 {
		t.Fatalf("extra arg: exit %d", code)
	}
	// --dry-run is sync-only
	if code := cmdAsset([]string{"list", "--dry-run"}); code != 1 {
		t.Fatalf("list --dry-run: exit %d", code)
	}
	// -h exits 0
	if code := cmdAsset([]string{"-h"}); code != 0 {
		t.Fatalf("-h: exit %d", code)
	}
}

func TestCmdAssetSyncNoConfig(t *testing.T) {
	dir := t.TempDir()
	if code := cmdAsset([]string{"sync", dir, "--dry-run"}); code != 1 {
		t.Fatalf("sync without rotor.config.ts: exit %d", code)
	}
}

func TestCmdAssetSyncNoAssetsSection(t *testing.T) {
	dir := t.TempDir()
	writeAssetFixture(t, dir, "rotor.config.ts", `
import { defineConfig } from "rotor/config";
export default defineConfig({});
`)
	if code := cmdAsset([]string{"sync", dir, "--dry-run"}); code != 1 {
		t.Fatalf("sync without assets section: exit %d", code)
	}
}

func TestCmdAssetSyncDryRun(t *testing.T) {
	dir := t.TempDir()
	writeAssetFixture(t, dir, "rotor.config.ts", `
import { defineConfig } from "rotor/config";
export default defineConfig({
	assets: {
		paths: ["assets/**/*.png", "assets/**/*.ogg"],
		output: { luau: "src/shared/assets.luau", types: "src/shared/assets.d.ts" },
		creator: { type: "user", id: 1 },
	},
});
`)
	writeAssetFixture(t, dir, "assets/logo.png", "not really a png")
	writeAssetFixture(t, dir, "assets/sounds/hit.ogg", "not really an ogg")
	writeAssetFixture(t, dir, "assets/notes.txt", "skipped")

	if code := cmdAsset([]string{"sync", dir, "--dry-run"}); code != 0 {
		t.Fatalf("dry run: exit %d", code)
	}

	// A dry run must not upload, lock, or generate anything.
	for _, rel := range []string{"rotor-lock.json", "src/shared/assets.luau", "src/shared/assets.d.ts"} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); err == nil {
			t.Fatalf("dry run wrote %s", rel)
		}
	}
}

func TestCmdAssetList(t *testing.T) {
	dir := t.TempDir()
	// empty (missing lockfile) is fine
	if code := cmdAsset([]string{"list", dir}); code != 0 {
		t.Fatalf("list without lockfile: exit %d", code)
	}
	writeAssetFixture(t, dir, "rotor-lock.json", `{
  "version": 1,
  "assets": {
    "assets/logo.png": { "hash": "sha256:abcdef1234567890", "assetId": 123 }
  }
}`)
	if code := cmdAsset([]string{"list", dir}); code != 0 {
		t.Fatalf("list: exit %d", code)
	}
	// corrupt lockfile errors
	writeAssetFixture(t, dir, "rotor-lock.json", "{nope")
	if code := cmdAsset([]string{"list", dir}); code != 1 {
		t.Fatalf("list with corrupt lockfile: exit %d", code)
	}
}

func writeAssetFixture(t *testing.T, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
