<p align="center">
  <img src="media/logo.png" alt="rotor — an all in one roblox toolchain" width="480">
</p>

<p align="center"><em>TypeScript in, Roblox out — at native speed.</em></p>

<p align="center">
  <a href="https://github.com/uproot/rotor/releases/latest"><img src="https://img.shields.io/github/v/release/uproot/rotor" alt="latest release"></a>
  <a href="https://github.com/uproot/rotor/actions/workflows/ci.yml"><img src="https://github.com/uproot/rotor/actions/workflows/ci.yml/badge.svg" alt="ci"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="MIT license"></a>
</p>

rotor is an all-in-one Roblox toolchain, written in Go. At its core is a native rewrite of the [roblox-ts](https://roblox-ts.com) compiler built on [typescript-go](https://github.com/microsoft/typescript-go) — a drop-in `rbxtsc` replacement with **byte-identical Luau output** — plus a native Luau toolchain and Open Cloud asset + deployment pipelines, all in one binary.

```sh
bun add -d @rotor-rbx/rotor    # or: npm i -D @rotor-rbx/rotor — see Install for more ways
```

📖 [Documentation](docs.md) · 🤝 [Contributing](CONTRIBUTING.md) · 🗺️ [Roadmap](roadmap.md)

## Features

**Compile & check** — the rbxtsc replacement:

```sh
rotor check ./my-game -w     # native full-strictness typecheck: 222 files in 161 ms
rotor build ./my-game -w     # byte-identical Luau, watch mode, incremental rebuilds
rotor doctor                 # diagnose tsconfig, @rbxts packages, plugins, Rojo, cloud setup
```

Same `tsconfig.json`, same `@rbxts/*` packages, same transformer plugins (Flamework etc.), same CLI flags. Plus built-in extensions rbxtsc doesn't have: **`$env`** (compile-time environment variables from `.env` — `$env.GAME_NAME`, `$env("KEY", "fallback")`, no plugin needed) and **`$getModuleTree` on folders** (no `index.ts` required).

**Luau toolchain** — works on any Rojo project, no rbxts required:

```sh
rotor dev                    # watch + incremental compile + rojo serve to Studio
rotor bundle entry.luau -o bundle.luau --minify   # inline a require graph, still runnable
rotor minify file.luau       # strip comments/whitespace, keep --! directives
rotor pack --as luau         # whole project -> one self-reconstructing script (or rbxm/rbxmx)
rotor sourcemap -o sourcemap.json                 # Rojo-compatible, for luau-lsp
```

**Cloud** — assets and deployment from one typed config (see below):

```sh
rotor asset sync             # upload new/changed assets, lockfile, typed codegen
rotor deploy plan -e prod    # diff config vs live state (terraform-style, no network writes)
rotor deploy apply -e prod   # publish places, settings, badges — only what drifted
```

**Scaffolding** — `rotor init` runs an interactive wizard (template, Biome/oxlint, starter packages, asset/deploy config) or scripts cleanly with `--yes`/`--template`.

## Configuration — `rotor.config.ts`

One typed TypeScript config drives the cloud tools. rotor evaluates it natively (no Node needed), generates `rotor-config.d.ts` for editor typing, and refuses npm imports — it's config, not a program:

```ts
import { defineConfig } from "@rotor-rbx/rotor";

export default defineConfig({
	assets: {
		paths: ["assets/**/*.png", "assets/**/*.ogg"],
		output: { luau: "src/shared/assets.luau", types: "src/shared/assets.d.ts" },
		creator: { type: "group", id: 12345 },
	},
	deploy: {
		environments: {
			dev: {
				universeId: 111,
				places: { start: { file: "build/game.rbxl", placeId: 222 } },
			},
			prod: {
				universeId: 333,
				places: { start: { file: "build/game.rbxl", placeId: 444, name: "Start", maxPlayers: 30 } },
				experience: { name: "My Game", playability: "public", privateServers: { price: 100 } },
				badges: { winner: { name: "Winner!", description: "Beat the game", icon: "assets/badge.png" } },
				gamePasses: { vip: { name: "VIP", price: 250, icon: "assets/vip.png" } },
				icon: "assets/icon.png",
				thumbnails: ["assets/thumb1.png", "assets/thumb2.png"],
				products: { coins: { name: "100 Coins", price: 25 } },
				socialLinks: { discord: { title: "Join us", url: "https://discord.gg/x", type: "discord" } },
			},
		},
	},
});
```

- **`rotor asset sync`** scans the globs, uploads new/changed files via Open Cloud (SHA-256 lockfile `rotor-lock.json` — unchanged files never re-upload, updates keep asset ids stable), and generates `assets.luau` + `assets.d.ts`, so code references `assets.sounds.hit` instead of hardcoded `rbxassetid://` strings.
- **`rotor deploy`** is infrastructure-as-code: it diffs the config against per-environment state (`.rotor/deploy/<env>.json`), shows a plan, and applies only the drift — place file publishing + place settings, experience settings, badges and game passes (icons upload automatically first, shared icons dedupe), experience icon + thumbnails, developer products, and social links. Deletes require `--allow-deletes`.
- Auth is an Open Cloud key in **`ROBLOX_API_KEY`** (scopes: Assets R/W, Universe Places W, Universe R/W). `rotor doctor` checks your config and key setup.
- Compile-time env vars come from `.env` / `.env.<NODE_ENV>` next to your tsconfig and are inlined by the `$env` macro; rotor writes `rotor-env.d.ts` so your editor sees the types.

Full config shape and every command flag: [docs.md](docs.md).

## Install

Grab a binary from [GitHub Releases](https://github.com/uproot/rotor/releases), or use a toolchain manager:

```sh
# mise
mise use -g github:uproot/rotor@1.5.0

# rokit
rokit add uproot/rotor@1.5.0
```

```toml
# aftman.toml
[tools]
rotor = "uproot/rotor@1.5.0"

# foreman.toml
[tools]
rotor = { github = "uproot/rotor", version = "1.5.0" }
```

### Install via npm / bun

For rbxts projects that already live in the JS ecosystem, install [`@rotor-rbx/rotor`](https://www.npmjs.com/package/@rotor-rbx/rotor) as a dev dependency — a postinstall step downloads the prebuilt binary for your platform:

```sh
bun add -d @rotor-rbx/rotor
npm i -D @rotor-rbx/rotor
pnpm add -D @rotor-rbx/rotor
yarn add -D @rotor-rbx/rotor
```

Installing straight from GitHub works too: `bun add -d github:uproot/rotor` (npm/pnpm/yarn equivalents likewise).

> **bun note:** bun skips postinstall scripts by default. Either add `"trustedDependencies": ["@rotor-rbx/rotor"]` to your project's `package.json` (then `bun install`), or do nothing — the `rotor` shim downloads the binary on first run. pnpm similarly asks you to approve build scripts (`pnpm approve-builds`), with the same first-run fallback.

Or build from source (Go 1.25+):

```sh
git clone https://github.com/uproot/rotor && cd rotor
go build ./cmd/rotor
```

## Benchmarks

Measured on real production rbxts games, with output byte-identical to `rbxtsc` 3.0.0:

| Workload | rotor |
|----------|------:|
| Full strict typecheck — 222-file production game | **161 ms** |
| Full build — 95-file production game | **355 ms** |
| Incremental watch rebuild — same game | **180 ms** |

The JS toolchain spends longer than this booting Node. The ~10× speedup is structural: rotor runs Microsoft's native, parallel TypeScript compiler ([typescript-go](https://github.com/microsoft/typescript-go)) instead of the single-threaded JS one.

## Contributors

<a href="https://github.com/uproot"><img src="https://github.com/uproot.png" width="56" height="56" alt="uproot"></a>
<a href="https://github.com/Coyenn"><img src="https://github.com/Coyenn.png" width="56" height="56" alt="Coyenn"></a>

Contributions welcome — see [CONTRIBUTING.md](CONTRIBUTING.md).

## License

[MIT](LICENSE). rotor stands on [roblox-ts](https://github.com/roblox-ts/roblox-ts) (MIT) and [typescript-go](https://github.com/microsoft/typescript-go) (Apache-2.0) — see [credits](docs.md#credits--licenses).
