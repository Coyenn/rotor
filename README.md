# ⚡ rotor

**A native-speed rewrite of the [roblox-ts](https://roblox-ts.com) compiler in Go — built on TypeScript's own native compiler.**

rotor is a drop-in replacement for `rbxtsc` that compiles TypeScript to Luau with **byte-identical output**, the same `@rbxts/*` npm ecosystem, and the same CLI — at roughly **10x the speed**.

> **Status: pre-alpha, under heavy development.** rotor already compiles a real production game — all 95 files — **byte-identical to `rbxtsc` 3.0.0**, and typechecks at native speed. The project layer (watch, incremental, full CLI) is next. Watch the roadmap. ⬇️

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

rotor already **compiles multi-file TypeScript projects to byte-identical Luau** — the full language surface: imports with Rojo-aware require chains, JSX (`@rbxts/react`), classes and decorators, async/generators, try/catch, enums and namespaces, spread, functions, closures, destructuring, the full macro tables (`Array.map`, `string.format`, `Map.get`, …), optional chaining, Map/Set/string/generator iteration, switch, `new` — verified continuously against real `rbxtsc` output (43/43 differential fixtures; **all 95 files of a real production game compile byte-identical**, zero divergent). It also **natively typechecks real rbxts projects** with watch mode.

### Build

Requires **Go 1.25+** (no Node needed to build or run rotor itself):

```powershell
git clone https://github.com/uproot/rotor && cd rotor
go build -o rotor.exe ./cmd/rotor
```

### Use it

Two commands so far:

```powershell
rotor check path/to/your-game        # native, full-strictness typecheck: diagnostics + timing
rotor check path/to/your-game -w     # watch mode: rechecks on save
rotor build path/to/your-game        # compile the project to Luau (supports --type, --noInclude, --includePath)
```

- `path` is a project directory containing a `tsconfig.json` (defaults to the current directory).
- Your project needs `node_modules` installed (rotor reads the same `@rbxts` types).
- Exit codes: `0` = no errors, `1` = errors found, `2` = usage/config failure — suitable for CI.

`rotor build` compiles every file in the project, writes the `.luau` outputs to your tsconfig's `outDir` exactly where `rbxtsc` would put them, and copies `include/` (RuntimeLib.lua, Promise.lua — verbatim from roblox-ts). Try it on rotor's own test fixture project to see it in action:

```powershell
rotor build testdata/diff/project
# out/01_literals.luau
# ...
# compiled 43 files in 189 ms
```

Caveats while the port is still in progress (see the [roadmap](roadmap.md)):

- The transformer covers the full language surface (JSX, classes, decorators, async/generators, try/catch, enums, namespaces, spread, the macro tables). Anything not yet ported fails loudly with a clear "not yet supported" diagnostic — rotor **never silently emits wrong output**. Everything that compiles is byte-identical to `rbxtsc` 3.0.0.
- There's no watch/incremental mode for `build`, no `.d.ts` emit for packages, and no transformer-plugin support yet — that's the Phase 4 project layer.

A standalone `.ts` file isn't compilable by itself — like `rbxtsc`, rotor needs the rbxts project around it (`package.json` with `@rbxts/compiler-types` + `@rbxts/types` installed, `tsconfig.json`, `default.project.json`). The fixture project above is a minimal working example of that setup.

## Roadmap

| Phase | Scope | Status |
|:-----:|-------|:------:|
| **0** | Foundation — Go module, vendored typescript-go mirror, TypeChecker driven from Go | ✅ |
| **1** | Luau AST + renderer — full port of `@roblox-ts/luau-ast` (40 node kinds, temp-id solver, byte-exact formatting) | ✅ |
| **2** | Transformer core — `TransformState`, prereq statement stack, core expression/statement transforms, **differential harness vs rbxtsc** | ✅ |
| **2b** | Functions, arrows, destructuring, `for...of` (arrays), switch, loop closure semantics | ✅ |
| **3a** | Imports & module resolution (Rojo-aware requires, `TS.import`/`TS.getModule`, export-from), `new` + constructor macros, math-op macros | ✅ |
| **3b** | Macro tables (`Array`/`String`/`Set`/`Map`/`Promise` + call macros), optional chaining, full Map/Set/string/generator iteration, pnpm symlink + `baseUrl` resolution | ✅ |
| **3c** | JSX (`@rbxts/react`), classes, decorators, object/array/call spread + logical assignments, async/generators, try/catch flow rerouting, enums, namespaces | ✅ |
| **4** | Project layer — `.d.ts` emit, incremental builds, watch, full `rbxtsc` CLI, transformer-plugin sidecar | 🚧 |
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

### Running the tests

```powershell
go test ./internal/... -count=1                           # full test suite
go test ./internal/diff/ -v -run TestDifferential          # differential suite only (see below)
go test ./internal/luau/render/ -bench . -benchmem        # renderer benchmarks
go test ./internal/spike/ -v                              # checker integration spike
go vet ./internal/...                                     # required clean before commits
```

No Node required to run any of the above — the rbxtsc goldens are committed.

### The differential suite (how rotor proves byte-parity)

`internal/diff` compiles every fixture project under `testdata/diff/project/src/` with rotor and byte-compares the output against committed goldens in `testdata/diff/golden/` — which were generated by the **real `rbxtsc` 3.0.0**. A fixture passes only when rotor's output is byte-identical; the first diverging line is reported.

Adding a fixture:

1. Write the TypeScript in `testdata/diff/project/src/` (it must compile cleanly under rbxtsc).
2. Regenerate goldens: `powershell -File tools/oracle/oracle.ps1` (this is the only step that needs Node/npm — it runs the pinned `roblox-ts@3.0.0` over the fixture project).
3. Enable the fixture in `internal/diff/manifest.go` and run `go test ./internal/diff/ -v`.

Existing goldens must stay byte-unchanged when regenerating — `git diff testdata/diff/golden/` should only show your new files.

### Project docs

- Design doc: [`docs/superpowers/specs/2026-06-05-rotor-design.md`](docs/superpowers/specs/2026-06-05-rotor-design.md)
- Full roadmap with every phase and task: [`roadmap.md`](roadmap.md)
- Phase plans: `docs/superpowers/plans/` · porting digests (the transformer's source of truth): `docs/superpowers/research/`

## Credits & licenses

rotor stands on two giants:

- [**roblox-ts**](https://github.com/roblox-ts/roblox-ts) (MIT) — the original compiler, whose emit semantics rotor faithfully ports. Vendored reference sources in `reference/` retain their MIT license.
- [**typescript-go**](https://github.com/microsoft/typescript-go) (Apache-2.0) — Microsoft's native TypeScript compiler. The vendored mirror in `tsgo/` retains its license and NOTICE; see `tsgo/MIRROR.md` for provenance and the statement of changes.

rotor itself is [MIT licensed](LICENSE).
