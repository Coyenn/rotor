package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"rotor/internal/assets"
	"rotor/internal/cloud"
	"rotor/internal/config"
	"rotor/internal/term"
)

// cmdAsset is `rotor asset <sync|list>`: an asphalt-style asset pipeline.
// `sync` scans the globs configured under `assets` in rotor.config.ts,
// uploads new/changed files via Open Cloud, records ids in rotor-lock.json,
// and regenerates the typed accessor modules (assets.luau + assets.d.ts).
// `list` prints the lockfile. `--dry-run` shows the sync plan and stops
// before any upload (no API key needed).
func cmdAsset(args []string) int {
	sub := ""
	dir := ""
	dryRun := false
	for _, a := range args {
		switch {
		case a == "-h" || a == "--help":
			assetUsage(os.Stdout)
			return 0
		case a == "--dry-run":
			dryRun = true
		case strings.HasPrefix(a, "-"):
			fmt.Fprintf(os.Stderr, "rotor asset: unknown flag %q\n\n", a)
			assetUsage(os.Stderr)
			return 1
		default:
			switch {
			case sub == "":
				sub = a
			case dir == "":
				dir = a
			default:
				fmt.Fprintf(os.Stderr, "rotor asset: unexpected extra argument %q\n\n", a)
				assetUsage(os.Stderr)
				return 1
			}
		}
	}
	if sub == "" {
		fmt.Fprintln(os.Stderr, "rotor asset: expected a subcommand (sync or list)")
		fmt.Fprintln(os.Stderr)
		assetUsage(os.Stderr)
		return 1
	}
	if dir == "" {
		dir = "."
	}
	switch sub {
	case "sync":
		return assetSync(dir, dryRun)
	case "list":
		if dryRun {
			fmt.Fprintln(os.Stderr, "rotor asset: --dry-run only applies to sync")
			return 1
		}
		return assetList(dir)
	default:
		fmt.Fprintf(os.Stderr, "rotor asset: unknown subcommand %q (want sync or list)\n\n", sub)
		assetUsage(os.Stderr)
		return 1
	}
}

func assetUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  rotor asset sync [path] [--dry-run]")
	fmt.Fprintln(w, "      scan the asset globs from rotor.config.ts, upload new/changed files")
	fmt.Fprintln(w, "      via Open Cloud (decals + audio), record ids in rotor-lock.json, and")
	fmt.Fprintln(w, "      regenerate the typed accessor modules (assets.luau / assets.d.ts);")
	fmt.Fprintln(w, "      --dry-run prints the plan without uploading (no API key needed)")
	fmt.Fprintln(w, "  rotor asset list [path]")
	fmt.Fprintln(w, "      print the lockfile (path, asset id, content hash)")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Uploading needs ROBLOX_API_KEY (an Open Cloud key with asset read/write scopes).")
}

// assetSync implements `rotor asset sync`.
func assetSync(dir string, dryRun bool) int {
	s := term.For(os.Stdout)

	cfg, err := config.Load(dir)
	if errors.Is(err, config.ErrNotFound) {
		fmt.Fprintf(os.Stderr, "rotor asset: no rotor.config.ts found in %q (asset sync needs an `assets` section)\n", dir)
		return 1
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "rotor asset: %v\n", err)
		return 1
	}
	for _, w := range cfg.Warnings {
		fmt.Fprintf(os.Stderr, "rotor asset: warning: %s\n", w)
	}
	if cfg.Assets == nil || len(cfg.Assets.Paths) == 0 {
		fmt.Fprintln(os.Stderr, "rotor asset: rotor.config.ts has no `assets` section (or assets.paths is empty)")
		return 1
	}

	scan, err := assets.Scan(dir, cfg.Assets.Paths)
	if err != nil {
		fmt.Fprintf(os.Stderr, "rotor asset: %v\n", err)
		return 1
	}
	lock, err := assets.LoadLockfile(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "rotor asset: %v\n", err)
		return 1
	}
	plan := assets.BuildPlan(scan, lock)

	// Print the plan: per-file lines for changes and skips, then the tally.
	for _, p := range plan.Skipped {
		fmt.Printf("  %s  %s %s\n", s.WarnBold(s.Glyphs().Warn), p, s.Muted("(unknown extension, skipped)"))
	}
	for _, it := range plan.Items {
		switch it.Action {
		case assets.ActionCreate:
			fmt.Printf("  %s %s %s\n", s.Green("+"), it.File.Path, s.Muted("(create, "+strings.ToLower(string(it.File.Type))+")"))
		case assets.ActionUpdate:
			fmt.Printf("  %s %s %s\n", s.Yellow("~"), it.File.Path, s.Muted(fmt.Sprintf("(update, asset %d)", it.AssetID)))
		}
	}
	creates := plan.Count(assets.ActionCreate)
	updates := plan.Count(assets.ActionUpdate)
	unchanged := plan.Count(assets.ActionUnchanged)
	fmt.Printf("  %s\n", joinDot(s, []string{
		s.Bold(fmt.Sprintf("%d to create", creates)),
		s.Bold(fmt.Sprintf("%d to update", updates)),
		s.Muted(fmt.Sprintf("%d unchanged", unchanged)),
		s.Muted(fmt.Sprintf("%d skipped", len(plan.Skipped))),
	}))

	if dryRun {
		fmt.Printf("  %s\n", s.Muted("dry run — nothing uploaded"))
		return 0
	}

	if plan.Changes() == 0 {
		if code := assetWriteOutputs(s, dir, cfg.Assets.Output, lock); code != 0 {
			return code
		}
		fmt.Printf("  %s  %s\n", s.SuccessBold(s.Glyphs().Check), s.Bold("everything up to date"))
		return 0
	}

	// Uploads ahead: validate the creator and build the cloud client.
	creator := cloud.Creator{}
	switch cfg.Assets.Creator.Type {
	case "user":
		creator.UserID = cfg.Assets.Creator.ID
	case "group":
		creator.GroupID = cfg.Assets.Creator.ID
	default:
		fmt.Fprintf(os.Stderr, "rotor asset: assets.creator.type must be \"user\" or \"group\" (got %q) in rotor.config.ts\n", cfg.Assets.Creator.Type)
		return 1
	}
	if cfg.Assets.Creator.ID == 0 {
		fmt.Fprintln(os.Stderr, "rotor asset: assets.creator.id is required in rotor.config.ts (the user or group that owns uploaded assets)")
		return 1
	}
	client, err := cloud.FromEnv()
	if errors.Is(err, cloud.ErrNoAPIKey) {
		fmt.Fprintln(os.Stderr, "rotor asset: ROBLOX_API_KEY is not set")
		fmt.Fprintln(os.Stderr, "  create an Open Cloud API key with the asset read/write scopes at")
		fmt.Fprintln(os.Stderr, "  https://create.roblox.com/dashboard/credentials and export it as ROBLOX_API_KEY")
		return 1
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "rotor asset: %v\n", err)
		return 1
	}

	res, err := assets.Sync(context.Background(), client, dir, plan, lock, assets.SyncOptions{
		Creator: creator,
		OnFile: func(item assets.PlanItem, assetID int64, err error) {
			if err != nil {
				fmt.Printf("  %s  %s %s\n", s.ErrorBold(s.Glyphs().Cross), item.File.Path, s.Error(err.Error()))
				return
			}
			fmt.Printf("  %s  %s %s\n", s.SuccessBold(s.Glyphs().Check), item.File.Path, s.Muted(fmt.Sprintf("rbxassetid://%d", assetID)))
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "rotor asset: %v\n", err)
		return 1
	}

	if code := assetWriteOutputs(s, dir, cfg.Assets.Output, lock); code != 0 {
		return code
	}

	parts := []string{
		s.Bold(fmt.Sprintf("%d created", res.Created)),
		s.Bold(fmt.Sprintf("%d updated", res.Updated)),
	}
	if len(res.Errors) > 0 {
		parts = append(parts, s.Error(plural(len(res.Errors), "failure")))
	}
	fmt.Printf("  %s\n", joinDot(s, parts))
	if len(res.Errors) > 0 {
		return 1
	}
	return 0
}

// assetWriteOutputs regenerates the configured codegen targets from the
// lockfile, printing what was written. Returns a process exit code.
func assetWriteOutputs(s *term.Styler, dir string, out config.AssetsOutput, lock *assets.Lockfile) int {
	if out.Luau == "" && out.Types == "" {
		return 0
	}
	if err := assets.WriteOutputs(dir, out.Luau, out.Types, lock); err != nil {
		fmt.Fprintf(os.Stderr, "rotor asset: %v\n", err)
		return 1
	}
	for _, p := range []string{out.Luau, out.Types} {
		if p != "" {
			fmt.Printf("  %s %s %s\n", s.Muted(s.Glyphs().Arrow), p, s.Muted("(generated)"))
		}
	}
	return 0
}

// assetList implements `rotor asset list`: a lockfile view.
func assetList(dir string) int {
	s := term.For(os.Stdout)
	lock, err := assets.LoadLockfile(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "rotor asset: %v\n", err)
		return 1
	}
	if len(lock.Assets) == 0 {
		fmt.Printf("  %s\n", s.Muted("no assets in "+assets.LockfileName+" (run `rotor asset sync` first)"))
		return 0
	}
	paths := make([]string, 0, len(lock.Assets))
	width := 0
	for p := range lock.Assets {
		paths = append(paths, p)
		if len(p) > width {
			width = len(p)
		}
	}
	sort.Strings(paths)
	for _, p := range paths {
		e := lock.Assets[p]
		fmt.Printf("  %-*s  %s  %s\n", width, p,
			s.Bold(fmt.Sprintf("rbxassetid://%d", e.AssetID)),
			s.Muted(shortHash(e.Hash)))
	}
	fmt.Printf("  %s\n", s.Muted(plural(len(paths), "asset")))
	return 0
}

// shortHash abbreviates "sha256:<64 hex>" for display.
func shortHash(h string) string {
	hex := strings.TrimPrefix(h, "sha256:")
	if len(hex) > 12 {
		hex = hex[:12]
	}
	return "sha256:" + hex
}
