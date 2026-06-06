# ⚡ rotor

**A native-speed rewrite of the [roblox-ts](https://roblox-ts.com) compiler in Go — built on TypeScript's own native compiler.**

rotor is a drop-in replacement for `rbxtsc` that compiles TypeScript to Luau with **byte-identical output**, the same `@rbxts/*` npm ecosystem, and the same CLI — at roughly **10x the speed**.

> **Status: pre-alpha, under heavy development.** rotor can already *typecheck* real rbxts projects at native speed (see below), but does not emit Luau yet — the transformer is being ported phase by phase. Watch the roadmap. ⬇️

```
$ rotor check ./my-game -w
rotor check — native TypeScript checking
checked 222 files in 161 ms — 0 errors
```

*That's a real rbxts game — 222 files, full strict typecheck, in the time a JS toolchain spends booting.*

---

## Why

roblox-ts is a brilliant compiler with one structural problem: it runs on the JavaScript TypeScript compiler API. Every build boots Node, parses, binds, and typechecks your entire project in single-threaded JS. Watch-mode rebuilds, cold builds, startup — all of it is slow, and it gets slower as your game grows.

You can't fix that with a syntax transpiler (SWC/esbuild-style), because roblox-ts's emit is **type-directed**: `for...of` compiles differently for an `Array` vs a `Map` vs a string, `+` becomes `+` or `..` by operand type, truthiness guards depend on whether a type can be `0` or `""`, and the entire macro system resolves through the type checker. No types, no correct Luau.

The unlock is [**typescript-go**](https://github.com/microsoft/typescript-go) — Microsoft's official native port of the full TypeScript compiler (shipping as TypeScript 7), ~10x faster with parallel checking. It's the only native implementation of the *real* checker in existence. rotor ports roblox-ts's emit layer to Go on top of it.

## How rotor stays 1:1

Compatibility isn't a hope — it's enforced by construction:

- **Differential testing**: every emitted `.lua` file is byte-compared against `rbxtsc` 3.0.0's output across the upstream test corpus *and real production games*, continuously in CI.
- **Behavioral conformance**: roblox-ts's ~486 runtime test cases, compiled by rotor and executed under [Lune](https://github.com/lune-org/lune).
- **Faithful porting**: the reference sources are vendored in-repo (`reference/`), and ports are reviewed line-by-line against them — down to quirks like ECMAScript `Number::toString` formatting and temp-identifier collision naming.
- **Same runtime**: `RuntimeLib.lua` and `Promise.lua` are reused verbatim from roblox-ts — zero behavioral drift at runtime.

Your existing project — `tsconfig.json`, `default.project.json`, `node_modules/@rbxts/*`, transformer plugins like Flamework — is the compatibility target, unchanged.

## Try it today

What works right now is the foundation: **native typechecking of real rbxts projects**, with watch mode.

```powershell
git clone <this repo> && cd rotor
go build -o rotor.exe ./cmd/rotor

rotor check path/to/your-game        # one-shot: diagnostics + timing
rotor check path/to/your-game -w     # watch mode: rechecks on save
```

Requires Go 1.25+. Your project needs `node_modules` installed (rotor reads the same `@rbxts` types).

## Roadmap

| Phase | Scope | Status |
|:-----:|-------|:------:|
| **0** | Foundation — Go module, vendored typescript-go mirror, TypeChecker driven from Go | ✅ |
| **1** | Luau AST + renderer — full port of `@roblox-ts/luau-ast` (40 node kinds, temp-id solver, byte-exact formatting) | ✅ |
| **2** | Transformer core — `TransformState`, prereq statement stack, core expression/statement transforms, **differential harness vs rbxtsc** | 🚧 |
| **3** | Type-directed layer — type predicates, `for...of` shapes, truthiness, all macros (`Array.map`, `string.format`, …), JSX, classes, async/generators/try | ⬜ |
| **4** | Project layer — Rojo resolution, path translation, imports, `.d.ts` emit, incremental builds, watch, full `rbxtsc` CLI, transformer-plugin sidecar | ⬜ |
| **5** | Conformance — full upstream behavioral suite under Lune, diagnostics corpus, byte-identical builds of real games | ⬜ |
| | **v1.0** — drop-in `rbxtsc` replacement | 🎯 |

## Architecture

```
your-game/src/**/*.ts
        │
        ▼
┌─────────────────────────────┐
│  typescript-go  (vendored)  │   real TS parser + binder + checker,
│  parse · bind · typecheck   │   native, parallel  (tsgo/)
└──────────────┬──────────────┘
               │  typed AST + TypeChecker queries
               ▼
┌─────────────────────────────┐
│  rotor transformer          │   port of roblox-ts's TSTransformer
│  TS AST ──► Luau AST        │   (internal/transformer — phase 2/3)
└──────────────┬──────────────┘
               ▼
┌─────────────────────────────┐
│  Luau AST + renderer        │   port of @roblox-ts/luau-ast
│  byte-exact Luau text       │   (internal/luau — done ✅)
└──────────────┬──────────────┘
               ▼
        out/**/*.lua   (+ RuntimeLib.lua, verbatim from roblox-ts)
```

- `tsgo/` — generated mirror of [microsoft/typescript-go](https://github.com/microsoft/typescript-go) internals (its packages are `internal/`-only upstream; the mirror rewrites import paths). Regenerate with `go run ./tools/mirror`. **Never edit by hand.**
- `reference/` — pinned roblox-ts v3.0.0 + luau-ast 2.0.0 sources: the porting reference and differential-test oracle.
- `internal/luau`, `internal/luau/render` — the Luau AST and renderer.
- `cmd/rotor` — the CLI.

## Development

```powershell
go test ./internal/...                                    # full test suite
go test ./internal/luau/render/ -bench . -benchmem        # renderer benchmarks
go test ./internal/spike/ -v                              # checker integration spike
```

Design doc: [`docs/superpowers/specs/2026-06-05-rotor-design.md`](docs/superpowers/specs/2026-06-05-rotor-design.md)

## Credits & licenses

rotor stands on two giants:

- [**roblox-ts**](https://github.com/roblox-ts/roblox-ts) (MIT) — the original compiler, whose emit semantics rotor faithfully ports. Vendored reference sources in `reference/` retain their MIT license.
- [**typescript-go**](https://github.com/microsoft/typescript-go) (Apache-2.0) — Microsoft's native TypeScript compiler. The vendored mirror in `tsgo/` retains its license and NOTICE; see `tsgo/MIRROR.md` for provenance and the statement of changes.

rotor itself is [MIT licensed](LICENSE).
