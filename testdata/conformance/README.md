# Conformance corpus (Phase 5)

A second differential corpus: roblox-ts's **own upstream test sources**, compiled
by the real `rbxtsc` 3.0.0 into committed goldens that rotor's output is
byte-compared against (`internal/conformance`). Everything is currently
**disabled** — `internal/conformance/manifest.go` has an empty `EnabledFixtures`
list, and the test skips (without even compiling the project) until Phase 5
starts enabling fixtures.

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
`go test ./internal/conformance/` compiles `project/` once via
`compile.CompileProject` and byte-compares each enabled out-file against its
golden; disabled goldens are logged as skipped with a count.

**Caveat:** `CompileProject` is all-or-nothing — any transformer diagnostic in
*any* corpus file aborts the whole compile. So enabling even one fixture
requires the entire 44-file corpus to transform diagnostic-free (byte parity
is still only checked for enabled entries). As of creation, the corpus
type-checks fully under rotor's embedded tsgo; the remaining blockers are
transformer NYS diagnostics (enums, spread, generators/yield).
