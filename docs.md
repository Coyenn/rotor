# rotor documentation

rotor is an all-in-one Roblox toolchain, written in Go. At its core is a native-speed rewrite of the [roblox-ts](https://roblox-ts.com) compiler — built on TypeScript's own native compiler — alongside a native Luau bundler, minifier, dev loop, and packer (`bundle`, `minify`, `dev`, `pack`).

rotor targets `rbxtsc` compatibility: **byte-identical Luau output**, the same `@rbxts/*` npm ecosystem, and the same CLI shape, at roughly **10x the speed** on the native TypeScript compiler.

- [Why](#why)
- [How rotor stays 1:1](#how-rotor-stays-11)
- [What works today](#what-works-today)
- [Commands](#commands)
- [Build options](#build-options)
- [Production readiness](#production-readiness)
- [Architecture](#architecture)
- [Roadmap](#roadmap)
- [Credits & licenses](#credits--licenses)

## Why

roblox-ts is a brilliant compiler with one structural problem: it runs on the JavaScript TypeScript compiler API. Every build boots Node, parses, binds, and typechecks your entire project in single-threaded JS. Watch-mode rebuilds, cold builds, startup — all of it is slow, and it gets slower as your game grows.

You can't fix that with a syntax transpiler (SWC/esbuild-style), because roblox-ts's emit is **type-directed**: `for...of` compiles differently for an `Array` vs a `Map` vs a string, `+` becomes `+` or `..` by operand type, truthiness guards depend on whether a type can be `0` or `""`, and the entire macro system resolves through the type checker. No types, no correct Luau.

The unlock is [**typescript-go**](https://github.com/microsoft/typescript-go) — Microsoft's official native port of the full TypeScript compiler (shipping as TypeScript 7), ~10x faster with parallel checking. It's the only native implementation of the *real* checker in existence. rotor ports roblox-ts's emit layer to Go on top of it.

## How rotor stays 1:1

Compatibility isn't a hope — it's enforced by construction:

- **Differential testing**: every emitted `.luau` file is byte-compared against `rbxtsc` 3.0.0's output — 43 committed fixture goldens run on every `go test`, and a real 95-file production game compiles 95/95 byte-identical.
- **Behavioral conformance**: roblox-ts's vendored runtime suite, compiled by rotor and executed under [Lune](https://github.com/lune-org/lune). The in-repo corpus and harnesses (`testdata/conformance`, `internal/conformance`) are fully enabled today: all 44 upstream golden fixtures are byte-for-byte green, the full vendored diagnostics corpus passes, the Lune suite currently reports `460 passed, 0 failed, 0 skipped`, and the real `randomness` acceptance compare is byte-for-byte green when pointed at a local checkout.
- **Faithful porting**: the reference sources are vendored in-repo (`reference/`), and ports are reviewed line-by-line against them — down to quirks like ECMAScript `Number::toString` formatting and temp-identifier collision naming.
- **Same runtime**: `RuntimeLib.lua` and `Promise.lua` are reused verbatim from roblox-ts — zero behavioral drift at runtime.

Your existing project — `tsconfig.json`, `default.project.json`, `node_modules/@rbxts/*`, transformer plugins like Flamework — is the compatibility target, unchanged.

## What works today

rotor **compiles multi-file TypeScript projects to byte-identical Luau** across the full language surface: imports with Rojo-aware require chains, JSX (`@rbxts/react`), classes and decorators, async/generators, try/catch, enums and namespaces, spread, functions, closures, destructuring, the full macro tables (`Array.map`, `string.format`, `Map.get`, ...), optional chaining, Map/Set/string/generator iteration, switch, `new` — verified continuously against real `rbxtsc` output (43/43 differential fixtures; **all 95 files of a real production game compile byte-identical**, zero divergent). It also **natively typechecks and watches real rbxts projects**.

Anything not yet ported fails loudly with a clear "not yet supported" diagnostic — rotor **never silently emits wrong output**. Everything that compiles is byte-identical to `rbxtsc` 3.0.0.

### rotor extensions (superset of rbxtsc)

These compile under rotor but not under rbxtsc; everything rbxtsc accepts is still byte-identical:

- **`$getModuleTree` on folders** — rbxtsc requires the specifier to resolve as a module, so pointing it at a folder only works if the folder has an `index.ts`. rotor resolves folder specifiers directly: relative ones (`"./systems"`) against the importing file, non-relative ones against `baseUrl`/`paths` (`"shared/systems"`) and then the project root (`"src/shared/systems"`). The usual server-import/isolation guards still apply.

## Commands

```
rotor check [path] [-w]       typecheck the project (native, full strictness)
rotor build [options] [path]  compile the project to Luau
rotor doctor [path]           diagnose the setup: tsconfig, @rbxts packages,
                              Node.js + transformer plugins, Rojo wiring
rotor minify <file> [-o out]  minify a Luau file (strips comments + whitespace,
                              keeps --! directives)
rotor bundle <entry> [-o out] [--minify]
                              inline a Luau require graph into one runnable file
rotor dev [path] [--no-serve] watch + incrementally compile, and serve to Studio
                              via `rojo serve` (the dev inner loop)
rotor pack [path] [--as luau|rbxmx|rbxm] [-o out] [--entry inst.path] [--rojo-tree]
                              package a Rojo project into one self-reconstructing
                              Luau script or a Roblox model file
```

- `path` is a project directory containing a `tsconfig.json` (defaults to the current directory).
- Your project needs `node_modules` installed (rotor reads the same `@rbxts` types).
- Exit codes: `0` = success, `1` = any failure (diagnostics, config, or usage) — matching upstream `rbxtsc`.
- Plugin-backed builds need Node.js at runtime for the transformer sidecar.

`rotor build` compiles every file in the project, writes the `.luau` outputs to your tsconfig's `outDir` exactly where `rbxtsc` would put them, runs the cleanup/copy pipeline, emits `.d.ts` files when `compilerOptions.declaration` is enabled, and copies `include/` (RuntimeLib.lua, Promise.lua — verbatim from roblox-ts). Try it on rotor's own test fixture project:

```powershell
bun install --cwd testdata/diff/project --no-save
rotor build testdata/diff/project
# out/01_literals.luau
# ...
# compiled 43 files in 189 ms
```

A standalone `.ts` file isn't compilable by itself — like `rbxtsc`, rotor needs the rbxts project around it (`package.json` with `@rbxts/compiler-types` + `@rbxts/types` installed, `tsconfig.json`, `default.project.json`). The fixture project above is a minimal working example of that setup.

## Build options

`rotor build` accepts the full rbxtsc flag surface (booleans accept `--flag`, `--flag=false`, `--no-flag`): `-p/--project`, `-w/--watch`, `--usePolling`, `--verbose`, `--noInclude`, `--logTruthyChanges`, `--writeOnlyChanged`, `--optimizedLoops`, `--type game|model|package`, `-i/--includePath`, `--rojo`, `--allowCommentDirectives`, `--luau`, plus rotor's own `--cpuprofile`. Run `rotor --help` for details.

Options may also be set under the top-level `"rbxts"` key of `tsconfig.json`; merge order: defaults < rbxts < command line.

## Production readiness

rotor is ready for production rbxts projects that want native-speed `check`, `check -w`, `build`, and `build -w`, including declaration emit, incremental rebuild selection, and transformer-plugin support through the bundled Node sidecar. Plugin-configured builds require Node.js on `PATH` so rotor can launch that sidecar.

Notes and current caveats (see the [roadmap](roadmap.md)):

- `build -w` reuses rotor's manifest-backed changed-file selection and runs a debounced, pruned polling watcher: `node_modules`, dot-directories, and the build-written `out`/`include` trees are never walked, editor write bursts ("save all") settle into one rebuild, edits made *during* a build are not lost, and editor junk files never trigger rebuilds. The poll adapts to the walk cost (100 ms floor), so idle watch CPU stays near zero even on big projects. Native FS events remain a possible future refinement.
- Declaration emit is available for declaration-enabled builds, but declaration-path alias rewriting still follows the current Phase 4 limitation called out in the roadmap.
- Transformer plugins run through the Node sidecar that ships **embedded in the rotor binary** (extracted on first plugin build). The worker uses your project's own `typescript` install — the same instance plugins `require` — and stays warm across builds and watch rebuilds. Validated against real `rbxts-transformer-flamework` and `rbxts-transform-env` packages.
- The conformance harnesses are in repo and green today. The external-project acceptance proof is environment-gated because it needs a local `randomness` checkout plus Rojo/Lune on the machine running it.

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
│  TS AST ──► Luau AST        │   (internal/transformer)
└──────────────┬──────────────┘
               ▼
┌─────────────────────────────┐
│  Luau AST + renderer        │   port of @roblox-ts/luau-ast
│  byte-exact Luau text       │   (internal/luau)
└──────────────┬──────────────┘
               ▼
        out/**/*.lua   (+ RuntimeLib.lua, verbatim from roblox-ts)
```

- `tsgo/` — generated mirror of [microsoft/typescript-go](https://github.com/microsoft/typescript-go) internals (its packages are `internal/`-only upstream; the mirror rewrites import paths). Regenerate with `go run ./tools/mirror`. **Never edit by hand.**
- `reference/` — pinned roblox-ts v3.0.0 + luau-ast 2.0.0 sources: the porting reference and differential-test oracle.
- `internal/luau`, `internal/luau/render` — the Luau AST and renderer.
- `internal/luau/lex`, `internal/luau/cst` — the Luau lexer and lossless CST/parser powering `minify`, `bundle`, and `pack`.
- `internal/version` — the single source of truth for rotor's release version.
- `cmd/rotor` — the CLI.

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
| **4** | Project layer — output pipeline, `.d.ts` emit, watch, plugin/concurrency integration | ✅ |
| **5** | Conformance — upstream behavioral suite under Lune, diagnostics corpus, acceptance closure | ✅ |
| | **v1.0** — drop-in `rbxtsc` replacement | ✅ |
| **v2** | Luau toolchain — lexer, lossless CST/parser, `minify`, `bundle`, `dev`, `pack` MVPs | ✅ |

The full roadmap with every phase and task lives in [`roadmap.md`](roadmap.md).

## Credits & licenses

rotor stands on two giants:

- [**roblox-ts**](https://github.com/roblox-ts/roblox-ts) (MIT) — the original compiler, whose emit semantics rotor faithfully ports. Vendored reference sources in `reference/` retain their MIT license.
- [**typescript-go**](https://github.com/microsoft/typescript-go) (Apache-2.0) — Microsoft's native TypeScript compiler. The vendored mirror in `tsgo/` retains its license and NOTICE; see `tsgo/MIRROR.md` for provenance and the statement of changes.

rotor itself is [MIT licensed](LICENSE).
