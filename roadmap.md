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
| **3c** | JSX, classes, decorators, spread, async, try, enums, namespaces | 🚧 |
| **4** | Project layer — emit layout, watch, incremental, full CLI, plugin sidecar | ⬜ |
| **5** | Conformance — upstream behavioral suite, diagnostics corpus, acceptance | ⬜ |
| | **v1.0 — drop-in `rbxtsc` replacement** | 🎯 |

**Measured progress:** 38/38 differential fixtures byte-identical to real rbxtsc 3.0.0;
`randomness` real-game smoke at **54/95 files byte-identical** (14 → 28 → 42 → 54 across
phases), zero divergent — every file that compiles is byte-exact.

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

## Phase 3c — JSX, Classes, Spread, Async, Try, Enums 🚧 (current)

*Plan: `docs/superpowers/plans/2026-06-07-rotor-phase3c.md`. Digests: `phase3c-jsx-digest.md`,
`phase3c-classes-digest.md`, `phase3c-async-try-enums-digest.md`. Breaks the JSX wall
(33 randomness files) and completes the language surface.*

- [x] **Task 1: JSX** — factory-call assembler, lowercase-tag quirk, attribute spread paths (`table.clone` fast path vs `_k`/`_v` loop), `{}` attr → `true`, JsxText fixup port from tsgo (backslash-doubling quirk), fixture `32_jsx.tsx`
- [x] **Task 2: Classes core** — setmetatable boilerplate byte-verbatim, `.new`/constructor synthesis, property initializers vs constructor-body order, parameter properties, static-method colon quirk, inheritance (`super()` 3 arms, `__tostring` re-emit), class expressions, computed names, `#field` diagnostic, fixture `33_classes.ts`
- [x] **Task 3: Decorators** — legacy experimental decorators (class/method/property/parameter, init-first-to-last apply-last-to-first order, `shouldInline` spill rules, key-pinning); acceptance: `@ReactComponent` error boundary; fixture `34_decorators.ts`
- [ ] **Task 4: Object/array spread + logical assignments** 🚧 — object spread fast path vs copy loop; array-literal spread; `??=` / `&&=` / `||=` at both dispatch points; fixture `35_spread.ts`
- [ ] **Task 5: async + generators** — `TS.async` wrappers (declarations become locals; async methods drop colon), `await`→`TS.await`, `TS.generator` body swap, `yield`/`yield*` lowering; fixture `36_async.ts`
- [ ] **Task 6: try/catch/finally + flow-control rerouting** — `TS.try` with `TRY_RETURN`/`TRY_BREAK`/`TRY_CONTINUE` flags, blocked checks, both load-bearing orderings, `collapseFlowControlCases`; retires the Phase 2 TRY_* no-op; fixture `37_try.ts`
- [ ] **Task 7: Enums + namespaces** — enum do-block with `_inverse` + setmetatable, const enums emit nothing, constant folding; namespace `_container` do-blocks, dotted/nested namespaces, merging banned (`noEnumMerging`/`noNamespaceMerging`); fixture `38_enums_namespaces.ts`
- [ ] **Task 8: Conformance + re-smoke + merge** — adversarial fixture `39_mixed3c.tsx` (feature interactions), full randomness re-smoke (expect a step change above 54/95), README/memory updates, final review, merge

## Phase 4 — Project Layer ⬜

*Scoped in the design spec; detailed plan not yet written. Everything that makes rotor a
usable CLI tool rather than a compile library.*

- [ ] Full emit layout — write `out/` tree, `include/` (RuntimeLib.lua + Promise.lua verbatim), `index.*` ↔ `init.*` translation, `.lua`/`.luau` output selection
- [ ] `.d.ts` emit for Package projects
- [ ] Full `rbxtsc` CLI flag surface — `build`, `-w`, `--type`, `--luau`, `--logTruthyChanges`, `--allowCommentDirectives`, `--writeOnlyChanged`, `--optimizedLoops` (currently always-on — gains its option), comment-directive hoisting (`--!strict` above header)
- [ ] Watch mode — native fs events, debounced batching
- [ ] Incremental builds (tsbuildinfo-equivalent)
- [ ] Transformer-plugin Node sidecar — real JS `typescript` package, text→text boundary (Flamework-class plugins unmodified); projects without plugins never spawn Node
- [ ] `validateCompilerOptions` full port (rootDir/outDir landed in 3a; rest deferred here)
- [ ] Concurrency: restore parallel checker workers (per-checker alias-mark caches via `GetTypeCheckerForFile`); retire per-file Program creation and the package-level `TransformStatement` func var
- [ ] Known cleanup: `getLastToken` block-`}` trailing-comment handling

## Phase 5 — Conformance ⬜

*The 1:1 proof at full scale.*

- [ ] Behavioral suite: all **486 upstream TestEZ cases** compiled by rotor → `rojo build` → executed under **Lune**
- [ ] Diagnostics corpus: all **87 expected-error files** report the same diagnostic IDs at the same locations
- [ ] Differential run over roblox-ts's full `tests/src` corpus (~7k LOC)
- [ ] Acceptance: `rotor build` on `randomness` byte-identical to rbxtsc 3.0.0 and the game runs
- [ ] Fix divergences to zero

## v1.0 — Drop-in replacement 🎯

Success criteria (from the design spec):

1. Byte-identical output (header-normalized) vs rbxtsc 3.0.0 on the upstream corpus and `randomness`
2. All 486 behavioral cases pass under Lune; all 87 diagnostic expectations match
3. `rotor build` and `rotor -w` work as drop-in `rbxtsc` replacements with the same npm packages
4. Measured wins: ≥5x cold build, near-instant watch rebuilds

**Out of scope for v1:** Playground/VirtualProject, `--writeTransformedFiles`, `devlink`
