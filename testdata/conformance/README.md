# Conformance corpus (Phase 5)

A second differential corpus: roblox-ts's **own upstream test sources**, compiled
by the real `rbxtsc` 3.0.0 into committed goldens that rotor's output is
byte-compared against (`internal/conformance`). Phase 5 now has three harnesses:

- `TestDiagnosticsCorpus` compiles the vendored `excluded/diagnostics/*` files
  one by one and asserts rotor's diagnostic IDs.
- `TestConformance` compiles enabled golden fixtures one by one through temp
  projects rooted under `project/`, so unsupported nodes in unrelated upstream
  specs do not block the fixtures that already match.
- `TestBehavioralSuite` and `TestRandomnessAcceptance` are environment-gated
  runners for the upstream runtime suite and the real `randomness` project.

## Provenance

Sources copied verbatim from the vendored upstream checkout at
`reference/roblox-ts/tests/src` — roblox-ts **v3.0.0**, commit
`d1d5486094ac1d1821b7eb03ce0de0890d30b82e` (see `reference/VERSIONS.md`).

Upstream `tests/src` contains **131 files**, all accounted for:

| Destination | Files | What |
|---|---|---|
| `project/src/` | 45 | `main.server.ts`, `services.d.ts`, `helpers/**` (5), `tests/**` (38 spec files, ~5.8k LOC of feature coverage) |
| `excluded/diagnostics/` | 86 | the entire upstream `diagnostics/` directory (85 `.ts`/`.tsx` + its `README.md`) |
| **Goldens** | **44** | every `project/src` file except `services.d.ts` (declaration file, emits nothing) |

## Exclusions

- `excluded/diagnostics/**` (85 sources): these files **intentionally fail
  compilation** — upstream's test driver (`src/CLI/test.ts`) compiles each one
  individually and asserts that the expected diagnostic is produced. They can
  never be part of a clean-compile golden corpus. They are kept here (not
  dropped) so a future diagnostics-conformance harness can use them.

No other file failed: all 44 emitting sources compile clean under the setup
below, on the first full run.

## Fixture project recipe (`project/`)

- `package.json` — exact pins, reconciled between `testdata/diff/project` and
  upstream `tests/package.json`:
  - `roblox-ts@3.0.0`, `typescript@5.5.3`, `@rbxts/compiler-types@3.0.0-types.0`,
    `@rbxts/types@^1.0.800` (same as the diff project),
  - plus the upstream test deps as npm pins: `@rbxts/roact@1.4.4-ts.0`,
    `@rbxts/services@1.5.3`, `@rbxts/testez@0.4.2-ts.0` (upstream declares the
    first via `github:roblox-ts/rbx-roact`; its lockfile resolved the same
    1.4.4-ts.0, which also exists on npm).
  - Upstream's lockfile had *stale* `@rbxts/compiler-types@2.2.0-types.0` and
    `@rbxts/types@1.0.751`; we use the 3.0.0-matching compiler-types and a
    newer types floor, identical to the diff corpus. Goldens were generated
    with `@rbxts/types@1.0.925` installed; `project/package-lock.json` is
    committed (like the diff project's) so installs are reproducible.
- `tsconfig.json` — upstream `tests/tsconfig.json` verbatim (minus comments)
  plus `"include": ["src"]`. Differences from the diff project's tsconfig:
  `jsxFactory` is `Roact.jsx` (not `React.createElement`), `baseUrl: "src"` is
  set, and `incremental`/`tsBuildInfoFile` are omitted.
- `default.project.json` — **model**-type, same shape as the diff project's.
  Upstream uses a full DataModel (game) project that places
  `helpers/rojo/isolated.ts` in StarterGui; under a model project nothing is
  isolated, and all files (including that one) still compile clean. Only the
  `diagnostics/` rojo-dependent tests cared, and those are excluded anyway.
- `rbxtsc` is invoked with `--allowCommentDirectives` because the upstream
  sources use `@ts-ignore`/`@ts-expect-error`; upstream's own test driver sets
  `allowCommentDirectives: true` for the same reason. Note rotor currently
  never emits `noCommentDirectives` (the diagnostic exists in
  `internal/transformer/diagnostics.go` but is unwired), so this flag matches
  rotor's present behavior; if rotor ever wires that pre-emit check it must
  also honor an allowCommentDirectives-equivalent for this corpus.

## Regenerating goldens

```
powershell -File tools/oracle/conformance-oracle.ps1
```

(Requires Node/npm — mise-managed; the script invokes
`.\node_modules\.bin\rbxtsc.cmd` directly since `npx` may be off PATH. It
npm-installs on first run, cleans `project/out/`, compiles, and mirrors
`out/**/*.luau` into `golden/` preserving subdirectories.)

## Enabling fixtures

Add the golden-relative slash path (e.g. `"tests/array.spec.luau"`) to
`EnabledFixtures` in `internal/conformance/manifest.go`. Then
`go test ./internal/conformance/ -run TestConformance -count=1 -v` compiles
each enabled fixture through a temp project rooted under `project/`, preserving
the shared `node_modules` tree while limiting the compile to the selected
source plus shared helpers.

## Diagnostics corpus

Run:

```powershell
go test ./internal/conformance/ -run TestDiagnosticsCorpus -count=1
```

The harness installs `project/node_modules` on first use if needed, then
overlays each vendored diagnostics fixture under `project/src/__diagnostics`
and compiles it through the real conformance project config. Two Rojo-topology
fixtures (`noRojoData.ts`, `noIsolatedImport.ts`) and the pre-Phase-4
`noRobloxSymbolInstanceof.*` family are explicitly skipped with reasons.

## Runtime suite

Run:

```powershell
go test ./internal/conformance/ -run TestBehavioralSuite -count=1 -v
```

Requires both `rojo` and `lune` on PATH. The test skips cleanly if either
tool is missing. When available it:

1. builds `testdata/conformance/project` with `rotor`,
2. runs `rojo build` to produce a place file,
3. executes the upstream `reference/roblox-ts/tests/runTestsWithLune.lua`.

## randomness acceptance

Run:

```powershell
go test ./internal/conformance/ -run TestRandomnessAcceptance -count=1 -v
```

Set `ROTOR_RANDOMNESS_PATH` to the local project root before running. The test
skips when unset and otherwise drives `compile.CompileProject` over the real
project, surfacing the emitted-file count or the first failure.
