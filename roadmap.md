# rotor Roadmap

rotor is a native Go reimplementation of the roblox-ts compiler (`rbxtsc`), built on
[typescript-go](https://github.com/microsoft/typescript-go). The contract: **byte-identical
Luau output** vs roblox-ts 3.0.0 (header-normalized), the same `@rbxts/*` ecosystem, the
same CLI — at roughly 10x the speed.

- **Design spec:** `docs/superpowers/specs/2026-06-05-rotor-design.md`
- **Detailed plans:** `docs/superpowers/plans/`
- **Porting source of truth:** research digests in `docs/superpowers/research/` (authority order: `reference/` > digest > plan > tests)
- **Acceptance test:** the real `randomness` game project compiles byte-identical to rbxtsc 3.0.0

Legend: ✅ done · 🚧 in progress · ⬜ not started

## Status at a glance

| Phase | Scope | Status |
|:-----:|-------|:------:|
| **0** | Foundation — repo, tsgo mirror, checker spike | ✅ |
| **1** | Luau AST + renderer (port of `@roblox-ts/luau-ast` 2.0.0) | ✅ |
| **2** | Transformer core + differential harness vs real rbxtsc | ✅ |
| **2b** | Functions, destructuring, for-of (arrays), switch, loop closures | ✅ |
| **3a** | Imports, module resolution, Rojo resolver, `new` + constructor macros | ✅ |
| **3b** | Macro tables, optional chaining, full iteration, pnpm/baseUrl resolution | ✅ |
| **3c** | JSX, classes, decorators, spread, async, try, enums, namespaces | ✅ |
| **4** | Project layer — emit layout, watch, incremental, full CLI, plugin sidecar | ✅ |
| **5** | Conformance — upstream behavioral suite, diagnostics corpus, acceptance | ✅ |
| | **v1.0 — drop-in `rbxtsc` replacement** | ✅ |

**Measured progress:** 43/43 differential fixtures and 44/44 conformance goldens are
byte-identical to real rbxtsc 3.0.0; the vendored behavioral suite passes under Lune
(`460 passed, 0 failed, 0 skipped` on June 7, 2026); the full vendored diagnostics corpus
passes without skips; and the real `randomness` project compares byte-for-byte across
`out/` + `include/` (`95/95` files, zero divergent, zero blocked).

---

## Phase 0 — Foundation ✅

*Plan: `docs/superpowers/plans/2026-06-06-rotor-phase0-phase1.md`. De-risks the core bet:
driving tsgo's real TypeChecker from Go.*

- [x] **Task 1: Repo scaffolding** — Go module, layout (`cmd/`, `internal/`, `tools/`, `reference/`, `tsgo/`), README, LICENSE
- [x] **Task 2: Vendor reference sources** — roblox-ts 3.0.0 + luau-ast 2.0.0 snapshots into `reference/` (SHAs recorded, `.git` stripped, licenses retained)
- [x] **Task 3: tsgo mirror tool** — `tools/mirror`: snapshots a pinned typescript-go commit and rewrites `internal/...` import paths to importable `rotor/tsgo/...` paths (+ overlay system for unexported APIs, e.g. `tsgo/checker/rotor_exports.go`)
- [x] **Task 4: Run the mirror and build tsgo** — pinned @ `1f955e97`; vendored tree compiles
- [x] **Task 5: Checker spike** — parse + typecheck a file from Go; exercise `GetTypeAtLocation`, `GetSymbolAtLocation`, config loading, program creation

## Phase 1 — Luau AST + Renderer ✅

*Plan: same file as Phase 0. Pure code, no checker; fully unit-tested; byte-exact formatting.*

- [x] **Task 6: SyntaxKind + Node interfaces** — all 40 Luau node kinds
- [x] **Task 7: List** — doubly-linked statement lists
- [x] **Task 8: Node structs**
- [x] **Task 9: Factories with clone-on-reparent**
- [x] **Task 10: Type guards, validators, globals**
- [x] **Task 11: RenderState + render utilities**
- [x] **Task 12: solveTempIds** — scope-aware temp-identifier collision solver
- [x] **Task 13: Expression renderers**
- [x] **Task 14: Statement renderers + RenderAST**
- [x] **Task 15: Golden integration test + benchmark**

## Phase 2 — Transformer Core + Differential Harness ✅

*Plan: `docs/superpowers/plans/2026-06-06-rotor-phase2.md`. First byte-identical Luau;
the harness (real rbxtsc goldens via `tools/oracle/oracle.ps1`, manifest-gated diff tests)
becomes the backbone for every later phase.*

- [x] **Task 1: Fixture project, oracle runner, committed goldens** — `testdata/diff/project` (pinned roblox-ts@3.0.0 / typescript@5.5.3), fixtures 01–10, `tools/oracle/oracle.ps1`
- [x] **Task 2: Differential harness** — `internal/diff` manifest-gated byte-compare vs goldens
- [x] **Task 3: Diagnostics + DiagnosticService** — error/warning factories with exact upstream messages (66 byte-exact diagnostics by Phase 3a)
- [x] **Task 4: TransformState core** — prereq statement stack, capture, `pushToVar*`, getType cache, `valueToIdStr`, hoisting maps, RuntimeLib tracking
- [x] **Task 5: Per-file pipeline** — transformSourceFile, statement-list hoist merge, export shapes, `CompileFile`, tsconfig sanitizer (strips TS7-removed options: `downlevelIteration`, `baseUrl`, `moduleResolution: "Node"`)
- [x] **Task 6: Dispatch + literals** — first byte-identical goldens (numerics raw-text, string quote rules, templates, array/object pointers)
- [x] **Task 7: Identifiers, variable statements, expression statements** — symbol lookup, `undefined`→`nil`, hoist checks, export-let routing
- [x] **Task 8: Type predicates + truthiness** — `isDefinitelyType`/`isPossiblyType` combinators; `createTruthinessChecks` (`~= 0`, NaN, `~= ""` order; TS#32778 workaround; `logTruthyChanges`)
- [x] **Task 9: Binary, logical, assignment, unary** — type-directed `+` vs `..`, `===`→`==`, bitwise→`bit32.*`, compound assignment, logical chain flattening, `??`
- [x] **Task 10: Access + calls** — +1 array indexing with constant folding, LuaTuple `select`, `isMethod` (`:` vs `.`), ensureTransformOrder
- [x] **Task 11: Control flow** — if/while/do/C-style for incl. optimized numeric-for detection, break/continue/throw, return rules
- [x] **Task 12: Export shapes end-to-end** — four shapes, mutable-export forcing the exports-table shape
- [x] **Task 13: Conformance sweep** — adversarial fixtures 11–13 (edge numbers/strings/prereq torture), final review, merge

## Phase 2b — Functions, Destructuring, for-of, switch ✅

*Plan: `docs/superpowers/plans/2026-06-07-rotor-phase2b.md`.*

- [x] **Task 1: Fixtures + goldens** — 14–19 (functions, arrows, destructuring, for-of, switch, closures)
- [x] **Task 2: Reference walker + loop closure copies + case hoisting** — `eachSymbolReferenceInFile` port; the real closure-copy machinery replaces Phase 2's panic; fixes the body-write loop divergence
- [x] **Task 3: Functions** — declarations/expressions/arrows, parameters (defaults, rest, implicit `self`, `this` elision), bodiless overloads, `export default function`
- [x] **Task 4: Destructuring** — array/object binding + assignment patterns, nesting, defaults, omitted elements, swap pattern, `noSpreadDestructuring`
- [x] **Task 5: for-of (arrays)** — `for _, x in exp do` shapes, inline destructure fast path, builder dispatch table (non-array → clean diagnostics)
- [x] **Task 6: switch** — repeat-until-true wrapper, `_fallthrough` flag, clauses-after-default quirk, case-clause hoisting
- [x] **Task 7: Conformance sweep + merge** — adversarial fixture 20, first `randomness` real-world smoke (14/95 byte-identical; blocker table drove Phase 3 priorities)

## Phase 3a — Imports, Module Resolution, NewExpression ✅

*Plan: `docs/superpowers/plans/2026-06-07-rotor-phase3a.md`. Unblocked the 96% of real-world
files that were import-blocked.*

- [x] **Task 1: RojoResolver + PathTranslator ports** — `internal/rojo`; references vendored @ v1.1.0 (partitions LIFO, init/index renames, sub-extensions, isolated/network containers)
- [x] **Task 2: MacroManager centralization + constructor macros** — one component owning all macro identification; `new Array(n)`, `Set`/`Map` literal-vs-loop, `WeakSet`/`WeakMap`; `transformNewExpression` (`X.new(...)` fallback, `new Instance("Part")`); math-op property-call macros (silent-wrong-output bug found via randomness, fixed)
- [x] **Task 3: Project-aware compile + runtime-lib emission** — `CompileProject`, Model/Game/Package require shapes, RuntimeLibRbxPath diagnostics; checker-pool pinned to one checker (alias-marks are per-checker — Phase 4 perf item to restore parallelism)
- [x] **Task 4: Import declarations** — elision via `IsReferencedAliasDeclaration`, lazy TS.import, default-import semantics, namespace imports, `createImportExpression` (TS.getModule, relative/absolute chains)
- [x] **Task 5: Export-from + export assignment** — per-statement re-exports, `export *` star loop, `export =`
- [x] **Task 6: Multi-file differential fixtures** — 21–23; harness switched to project-wide compilation
- [x] **Task 7: Conformance + re-smoke + merge** — adversarial fixture 24; randomness 28/95; syntactic-diagnostics gap fixed

## Phase 3b — Resolution Gaps, Macro Tables, Optional Chaining, Iteration ✅

*Plan: `docs/superpowers/plans/2026-06-08-rotor-phase3b.md`. Digest: `phase3b-digest.md`.*

- [x] **Task 1: Resolution fixes** — sanitizer `baseUrl`→`paths` rewrite; compiler-types Iterable-arity d.ts overlay (TS5/TS7 divergence); macro registration audit (`Missing()`, sentinel-gated)
- [x] **Task 2: pnpm symlinks** — `GuessVirtualPath` via tsgo SymlinkCache (lexicographic-min; junction-safe Realpath)
- [x] **Task 3: Macro infrastructure + String/ArrayLike tables** — `wrapComments` (`-- ▼ X ▼` markers), `argumentsWithDefaults`, 13 String macros + ArrayLike.size
- [x] **Task 4: Array tables** — ReadonlyArray (15 macros) + Array (9 macros), verbatim emit logic
- [x] **Task 5: Set/Map/Promise + call macros** — Set/Map families, `Promise.then`→`andThen`, `assert`/`typeOf`/`typeIs`/`classIs`/`identity`/`$tuple`/`$range`, Promise identifier macro
- [x] **Task 6: for-of builders completion** — Set/Map (+inline `[k,v]`), string gmatch, IterableFunction, LuaTuple arity introspection, generator `.next`, `$range` numeric-for; all 7 binding accessors
- [x] **Task 7: Optional chaining** — snapshot-free chainItem, double nil-check nesting, `_self` method rule, temp reuse, `noOptionalMacroCall`
- [x] **Task 8: Conformance + re-smoke + merge** — fixture 31; 35/35 fixtures; randomness 54/95

## Phase 3c — JSX, Classes, Spread, Async, Try, Enums ✅

*Plan: `docs/superpowers/plans/2026-06-07-rotor-phase3c.md`. Digests: `phase3c-jsx-digest.md`,
`phase3c-classes-digest.md`, `phase3c-async-try-enums-digest.md`. Breaks the JSX wall
(33 randomness files) and completes the language surface.*

- [x] **Task 1: JSX** — factory-call assembler, lowercase-tag quirk, attribute spread paths (`table.clone` fast path vs `_k`/`_v` loop), `{}` attr → `true`, JsxText fixup port from tsgo (backslash-doubling quirk), fixture `32_jsx.tsx`
- [x] **Task 2: Classes core** — setmetatable boilerplate byte-verbatim, `.new`/constructor synthesis, property initializers vs constructor-body order, parameter properties, static-method colon quirk, inheritance (`super()` 3 arms, `__tostring` re-emit), class expressions, computed names, `#field` diagnostic, fixture `33_classes.ts`
- [x] **Task 3: Decorators** — legacy experimental decorators (class/method/property/parameter, init-first-to-last apply-last-to-first order, `shouldInline` spill rules, key-pinning); acceptance: `@ReactComponent` error boundary; fixture `34_decorators.ts`
- [x] **Task 4: Object/array spread + logical assignments** — object spread fast path vs copy loop (+ all 7 iterable-to-array builders, call-argument spread); array-literal spread (`table.move`, `_length` bookkeeping quirks); `??=` / `&&=` / `||=` at both dispatch points; fixture `35_spread.ts`
- [x] **Task 5: async + generators** — `TS.async` wrappers (declarations become locals; async methods drop colon), `await`→`TS.await`, `TS.generator` body swap, `yield`/`yield*` lowering, `async function*` banned; fixture `36_async.ts`
- [x] **Task 6: try/catch/finally + flow-control rerouting** — `TS.try` with `TRY_RETURN`/`TRY_BREAK`/`TRY_CONTINUE` flags, blocked checks, both load-bearing orderings, `collapseFlowControlCases`; retires the Phase 2 TRY_* no-op; fixture `37_try.ts`
- [x] **Task 7: Enums + namespaces** — enum do-block with `_inverse` + setmetatable, const enums emit nothing, constant folding; namespace `_container` do-blocks, dotted/nested namespaces, merging banned (`noEnumMerging`/`noNamespaceMerging`); fixture `38_enums_namespaces.ts`
- [x] **Task 8: Conformance + re-smoke + merge** — adversarial fixture `39_mixed3c.tsx` (decorated React class components, spread props, enum-keyed Map iteration into JSX children, generators with `yield*` in try spread into JSX, async + try/await + break/continue rerouting, `??=` on class fields, namespace components as JSX tags) byte-identical on first run; randomness re-smoke **95/95 byte-identical, zero divergent, zero blocked**; README/roadmap updated (final review + merge handled at branch close)

## Phase 4 — Project Layer ✅

*Plan: `docs/superpowers/plans/2026-06-07-rotor-phase4.md`. Digest: `phase4-project-digest.md`.
Everything that makes rotor a usable CLI tool rather than a compile library.*

- [x] Full emit layout — write `out/` tree, `index.*` ↔ `init.*` translation, cleanup/copyFiles passes landed in 4's output pipeline; `.lua`/`.luau` output selection landed via `--luau`; include/ emission had already landed in 3c (`internal/includefiles`, `--noInclude`/`--includePath`)
- [x] `.d.ts` emit for declaration-enabled builds — Package projects now emit declarations through tsgo's declaration pass, with the `types="types"` rewrite handled in Rotor's write callback
- [x] Full `rbxtsc` CLI flag surface — landed in 4 Task 1: ProjectOptions merge (defaults < tsconfig `rbxts` key < argv; absent CLI booleans don't clobber `rbxts` values), `-p/--project` file-path + upward tsconfig search, `--rojo` (empty-string falls through to discovery, quirk verbatim), `--luau`, `--logTruthyChanges`, `--optimizedLoops` (transformer gate wired), `--writeOnlyChanged` (cmd-level byte-compare; moves into the compile write phase with the output-pipeline task), `--verbose` + LogService analog (`internal/logservice`: yellow `Compiler Warning:` channel — now carries the previously-dropped Rojo resolver warnings — partial-line tracking, upstream benchmark/progress line formats), `--version`, and `--allowCommentDirectives`; usage errors now exit 1 for rbxtsc parity (was 2). Parsed-but-deferred: `--writeTransformedFiles` (warned NYS; out of v1). Comment-directive hoisting (`--!strict` above header) was already landed (`transformer/sourcefile.go`); ~~`build`, `--type`, `-w`, `--usePolling`~~ landed in 3c/4
- [x] Watch mode v1 — `rotor build -w` landed with polling-based rebuilds (`--usePolling` semantics effectively always-on today); build execution reuses the manifest-backed changed-file selection path. Superseded by **watch v2** (June 11, 2026 — see Post-v1 Follow-up): pruned `os.ReadDir`-based snapshots, debounced batch rebuilds, mid-build edit detection, adaptive 100 ms cadence
- [x] Incremental builds (tsbuildinfo-equivalent) — rotor now writes its own manifest at the configured buildinfo path and recompiles only changed `.ts`/`.tsx` files plus transitive importers; declaration emit follows the same selected set and failed builds do not advance the manifest
- [x] Transformer-plugin Node sidecar integration — plugin-configured compiles/builds run the worker from `tools/sidecar`, log `transformer-not-found` as warnings, hard-fail when Node is unavailable, and recompile/render from an overlay-backed tsgo Program. Hardened June 10, 2026: the worker is **embedded in the binary** (extracted to a content-addressed user-cache dir; `ROTOR_SIDECAR_PATH` overrides), resolves **typescript from the project's node_modules** (the same instance plugins require — upstream's shared-instance guarantee), reroutes plugin `console.log` off the stdout protocol, and rotor keeps **one warm worker per project across builds and `-w` rebuilds** with stamp-diffed `changedFiles` overlays. Proven against real `rbxts-transformer-flamework@1.3.2` + `rbxts-transform-env@3.0.0` (`testdata/transformers/project`, identifier metadata + `Flamework.addPaths` rojo rewriting + env inlining asserted). Note: `@rbxts/transform-env` does not exist on npm; the real package is `rbxts-transform-env`. Known limitation: a warm worker's plugin-visible view of an edited ambient `.d.ts` can be stale until the watch session restarts (rotor's own typecheck is unaffected)
- [x] `validateCompilerOptions` full port — landed in 3c (byte-exact diagnostic texts; known gap: enforced options set only in an `extends` parent are read root-only — same root-only gap as the sanitizer; fix with extends-chain resolution here)
- [x] Concurrency: restore checker-affined project compilation via `GetTypeCheckerForFile`; `CompileProject`/`CompileFile` no longer assume checker 0, project compilation now runs one transform worker per checker group with a per-checker `MultiState`, and the tsgo checker pool is no longer pinned to one checker
- [x] **Performance — filesystem-stat caching** (on top of the checker-group parallelism above; measured on the 95-file acceptance project, byte-identical output): full `rotor build` **545 ms → 355 ms**, incremental no-op rebuild (the watch case) **369 ms → 180 ms (~2×)**:
  - **`cachedvfs` host FS** — wraps the compiler host (build + `rotor check`) so module resolution's repeated `Stat`/`FileExists`/`Realpath`/`DirectoryExists` probes are memoized for the pass (was ~40% of warm time in `GetFileAttributesEx` syscalls; every checker group otherwise re-stats the same `node_modules/@rbxts` paths). Safe: a build never mutates its source tree mid-pass. Same wrapper tsgo's own project host uses.
  - **Rojo `searchDirectory`** — uses `os.ReadDir` entry kinds instead of an `os.Stat` per child ×2; only reparse points (junctions/symlinks) fall back to a following stat, so pnpm installs stay correct.
- [x] **CLI logging + DX** — new `internal/term` color/style layer (NO_COLOR/FORCE_COLOR aware, Windows VT enablement, glyphs with ASCII fallbacks) and `cmd/rotor/ui.go`: clean colored `build`/`check`/`watch` output (banner, ✓/✗ result blocks, throughput, colored failures, watch change/idle lines). `internal/logservice` left byte-stable for differential tests. New `--cpuprofile <path>` diagnostics flag.
- [x] V1 cleanup triage — the remaining non-surface cleanup (`TransformStatement` func-var removal) is tracked as post-v1 engineering follow-up rather than a parity blocker; warmer sidecar watch sessions landed June 10, 2026
- [x] Known cleanup: `getLastToken` block-`}` trailing-comment handling

## Phase 5 — Conformance ✅

*The 1:1 proof at full scale.*

- [x] Behavioral suite closure: the staged Lune harness now runs the full vendored runtime suite green (`460 passed, 0 failed, 0 skipped` on June 7, 2026), including the Roact JSX/runtime coverage
- [x] Diagnostics corpus closure: diagnostics harness now proves the full vendored expected-error corpus, including the two Rojo-topology fixtures (`noIsolatedImport.ts`, `noRojoData.ts`) via upstream-shaped temporary project staging
- [x] Differential run harness over roblox-ts's vendored `tests/src` corpus exists — `internal/conformance` now enables **44** golden fixtures byte-for-byte with **zero** manifest holdouts
- [x] Acceptance closure: the `randomness` acceptance runner now stages copied projects, reuses the real local dependency tree, runs Rotor and `rbxtsc`, and byte-compares normalized `out/` + `include/` artifacts against zero divergences
- [x] Close remaining runtime and acceptance divergences to zero

## v1.0 — Drop-in replacement ✅

Verified locally on June 7, 2026 with:

- `go test ./... -count=1`
- `go test ./internal/conformance -count=1` with `ROTOR_ROJO_PATH`, `ROTOR_LUNE_PATH`, and `ROTOR_RANDOMNESS_PATH`
- `bun test` in `tools/sidecar` (after `bun install --no-save`; `npm test` remains available as a fallback)
- `go test ./internal/compile -run TestTransformersFixture -count=1` — real-package transformer coverage (Flamework + rbxts-transform-env; requires `bun install --no-save` in `testdata/transformers/project`)

Continuously verified in CI (June 8, 2026): `.github/workflows/ci.yml` runs gofmt +
`go vet` + `go build ./...` + full `go test ./...` on every push to `master` and PR via
the shared `.github/actions/setup` composite action — which provisions Go, **Bun** (the
canonical fixture installer; npm fallback was the source of flaky installs) and Node (for
the transformer sidecar). CI also runs the sidecar JS suite (`node --test` in
`tools/sidecar`) and installs `testdata/transformers/project` so the real-package
transformer test runs. CI and `release.yml` set `ROTOR_REQUIRE_TRANSFORMERS_FIXTURE=1`
so a missing fixture install fails the run instead of silently skipping the
real-package test. rojo/lune are intentionally not installed in CI, so the
lune-executed runtime suite skips there; it runs locally via `aftman install` and is
exercised by the byte-parity differential/diagnostics tests regardless. `release.yml`
reuses the same setup, runs the tests, then publishes CLI binaries via GoReleaser.

Linux validation (June 10, 2026, golang:1.26-bookworm container + Node 22 + npm
installs from a clean clone): sidecar JS suite, `go vet`, full `go test ./...`,
sidecar/transformer tests under `-race`, and a release-style
(`CGO_ENABLED=0 -trimpath -ldflags "-s -w -X main.version=..."`) `rotor build` of the
transformers fixture — including embedded-sidecar extraction to `~/.cache/rotor` —
all green. All six GoReleaser targets cross-compile; `goreleaser check` validates the
config. Note: releases up to **v1.0.2 predate the embedded sidecar** — their
transformer-plugin support only worked from a repo checkout; the first release after
June 10, 2026 carries the fix.

Success criteria (from the design spec):

1. Byte-identical output (header-normalized) vs rbxtsc 3.0.0 on the upstream corpus and `randomness`
2. The vendored behavioral suite passes under Lune and the vendored diagnostics corpus matches upstream expectations
3. `rotor build` and `rotor build -w` work as drop-in `rbxtsc` replacements with the same npm packages
4. Measured wins: ≥5x cold build, near-instant watch rebuilds

**Out of scope for v1:** Playground/VirtualProject, `--writeTransformedFiles`, `devlink`

## Post-v1 Follow-up

**Watch v2 + QOL + hardening (June 11, 2026; spec: `docs/superpowers/specs/2026-06-11-rotor-watch-v2-qol-design.md`):**

- [x] **Watch engine v2** — pruned snapshot walks (`os.ReadDir` + `entry.Info()`; `node_modules`, dot-dirs, and the build-written `out`/`include` trees never walked — v1 stat'ed every `node_modules` file every 250 ms), editor-junk ignore (vim swap/backup probes, emacs locks, `.DS_Store`, `Thumbs.db`), debounced batch rebuilds (50 ms quiet timer, 500 ms cap — a "save all" burst lands as one rebuild reporting every file), pre-build baseline snapshots (fixes the v1 lost-update bug where a save during a build was silently absorbed), adaptive poll cadence (10× walk cost, clamped 100 ms–1 s; v1 was a fixed 250 ms). Applies to both `build -w` and `check -w`. Smoke-verified end to end: 43-file fixture, single-file edit → 1-file incremental rebuild in 64 ms
- [x] **Watch/CLI logging QOL** — batched change lines (`3 files changed · a.ts, b.ts, +1 more`), rebuild counter, last-build duration, and a build-time history sparkline (`term.Spark`, interactive terminals only) on the idle line
- [x] **`rotor doctor`** — new command diagnosing project setup with ✓/!/✗ rows + hints: tsconfig discovery, `node_modules`, `@rbxts/compiler-types`/`@rbxts/types`/`typescript` versions, Node.js (hard requirement only when transformer plugins are configured), per-plugin resolution, embedded-sidecar extraction, Rojo project file + CLI. Exit 1 only on hard failures
- [x] **Security hardening** — output-path containment guard (`filepath.IsLocal` on every compiled output's project-relative path before writing), sidecar cache extraction tightened to `0o700`/`0o600`, and a `vuln` CI job running pinned `govulncheck@v1.3.0 ./...` (validated locally: "No vulnerabilities found")

**v1.2.1 — transformer/render/resolution fixes (June 12, 2026):**

- [x] **Void expressions** — port `transformVoidExpression`: the operand is transformed in statement form (prereq'd for its side effects) and the expression evaluates to `nil`; byte-identical to rbxtsc v3.0.0 in both statement position (`void f()` → `f()` plus the discarded temp) and value position (`const x = void f()` → prereq `f()`, `local x = nil`)
- [x] **Empty-return render** — `return $tuple()` lowers to a `ReturnStatement` with an empty expression list; render it byte-exactly like upstream luau-ast (`return ` with trailing space) instead of treating it as an invariant violation and panicking with an internal compiler error. Other `renderExprOrList` callers (assignments, declarations) keep the invariant
- [x] **Symlink-cache resolution fix (tsgo)** — stop fabricating symlink cache entries for packages that resolved without a symlink (any plain npm/bun install). The bogus entries rebased every import of the affected package onto a nonexistent project-root path, surfacing as intermittent, map-iteration-order-dependent `noRojoData` failures; now matches the strada guard on `resolution.originalPath` truthiness

**Remaining:**

- ~~Keep one warm Node sidecar session across `build -w` rebuilds instead of respawning per polling cycle~~ — landed June 10, 2026 (transformer-plugin hardening; see Phase 4 sidecar entry)
- Retire the package-level `TransformStatement` func var now that the transform surface is feature-complete
- Fold the remaining root-only `extends`-chain validation/sanitizer limitation into the project-options pipeline
- Warm sidecar `.d.ts` staleness: stamp-diffed `changedFiles` cover the `.ts`/`.tsx` compile surface; an edited ambient `.d.ts` is invisible to *plugins* (not to rotor) until the watch session restarts

---

## Luau toolchain — v2 direction 🚧

*Spec: `docs/superpowers/specs/2026-06-12-rotor-luau-toolchain-design.md`. Expands rotor
from a `rbxtsc`-parity TS→Luau compiler into an all-in-one Luau toolchain — a
require-resolving **bundler** (still runnable, darklua-style), a **minifier**, and a
**`rotor dev`** watch+serve loop. References: darklua + full_moon. The existing
byte-parity compiler is untouched; this is additive. Four sub-projects, each with its
own plan.*

**Sub-project A — Luau front-end** (lexer + trivia-preserving CST + parser + generators; the critical path)

- [x] **A.1 — Lexer** (`internal/luau/lex`; plan: `docs/superpowers/plans/2026-06-12-rotor-luau-lexer.md`) — hand-written single-pass tokenizer that keeps whitespace/comment trivia with byte offsets + line/col. Core invariant: concatenating every token's text reproduces the source exactly. Full Luau lexical surface (long strings/comments with bracket levels, backtick interpolation with nested holes, `//`/`::`/`->`/compound-assign longest-match, numeric separators + `0x`/`0b` + hex floats without swallowing `..`). Permissive lexemes, non-panicking recovery. **Verified: 405 real Luau files roundtrip byte-exact (RuntimeLib, Promise, 44 conformance specs, 224 `@rbxts` package files); fuzzed 5.2M execs, 0 failures.**
- [ ] **A.2 — CST + parser** (`internal/luau/cst`) — Roslyn-style trivia attachment (`TokenRef` leading/trailing), surface-faithful node taxonomy, hand-written recursive-descent/Pratt parser with error recovery; byte-exact roundtrip via the generator
- [ ] **A.3 — Generators** (`internal/luau/gen`) — `readable` / `dense` (minified) / `retain_lines` serializers

**Sub-project B — `rotor dev`** (watch cwd + incremental build + supervise `rojo serve`; independent of A)

- [ ] `rotor dev` command

**Sub-project C — Minifier** (`rotor minify`; depends on A)

- [ ] Scope-aware `rename_variables`, `remove_comments`, dense output, `convert_index_to_field`

**Sub-project D — Bundler** (`rotor bundle`; depends on A + `internal/rojo`)

- [ ] Require-graph resolution (path + rojo modes) → module-table inlining with Roblox-faithful caching + recursive-require errors
