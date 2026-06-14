# rotor unified code-frame diagnostics — design

Date: 2026-06-14. Status: approved under the standing autonomous-execution grant.

## Goal

When user code fails to compile, rotor should **show the offending source** — the
line(s) of code, a caret/underline pointing at the span, and reserved keywords
highlighted — instead of a bare `path:line:col: message` one-liner.

Today only `rotor check` does this (via tsgo's `diagnosticwriter`). Every other
error path prints a flat message:

- `rotor build` — both TypeScript pre-emit errors **and** rotor's own
  transformer/macro diagnostics (`noAny`, `rotorNotYetSupported`, `$env`/`$asset`
  errors, …) surface as message-only strings. The build pipeline flattens
  diagnostics to `[]string` and the transformer→`DiagnosticInfo` conversion
  **drops the `Node`**, so position is lost before it reaches the CLI.
  (`internal/compile/compile.go` `transformAndRenderDetailed`,
  `internal/compile/project.go` `compiledProjectSourceFile.diags []string`.)
- `rotor bundle` / `rotor minify` / `rotor pack` — Luau parse/lex errors
  (`cst.Diagnostic{Pos{Offset,Line,Col}, Message}`) print as
  `path:line:col: message` (`cmd/rotor/minify.go:75`,
  `internal/bundle/bundle.go:86`, `internal/pack/luau.go`).

A single, language-agnostic **frame renderer** fixes all of these with one unit
of code, used by every error site across `build`/`bundle`/`minify`/`pack`.

## Decisions (settled during brainstorming)

1. **Unified rotor frame for both TypeScript and Luau** — one renderer, one
   visual style across the toolchain (not tsgo-style for TS + rotor-style for
   Luau).
2. **Highlight tier: keywords + caret** — show the source line(s) with a gutter
   and a severity-colored caret/underline, and color reserved keywords. *Not* a
   full tokenizer-driven syntax highlighter (no string/number/comment coloring).

## Scope & non-goals

**In scope:**

- A new `internal/diagframe` renderer package.
- Threading structured location (file + byte offset + length) through the
  `build`/compile diagnostic path so TS and transformer/macro errors can be
  framed.
- Wiring the renderer into `rotor build`, `rotor bundle`, `rotor minify`,
  `rotor pack`.

**Explicit non-goals:**

- **Changing `rotor check`'s output.** It keeps tsgo's `diagnosticwriter`. The
  conformance/differential harness asserts on tsgo's exact diagnostic
  formatting, and the byte-parity contract makes this the high-risk path to
  leave alone.
- **Changing diagnostic message or code text.** The frame is a presentation
  wrapper *around* the existing message; `noAny` still says exactly what it says
  today. The diff/conformance harness asserts codes and messages, which are
  preserved.
- **Changing generated `.luau` bytes.** This effort touches only error
  presentation on stderr.
- **Full syntax highlighting** (strings/numbers/comments). Deferred; the chosen
  tier is keywords-only.
- **Config-file errors** (`internal/config/load.go`'s `path:line:col` lines).
  Out of scope for this pass; the renderer is reusable there later if wanted.

## Architecture

### 1. `internal/diagframe` — the renderer

Pure, no filesystem I/O. The caller supplies the source text; the renderer
slices and decorates it. Uniform interface keyed on **byte offset** (both Luau
`cst.Diagnostic.Pos.Offset` and tsgo node/diagnostic positions are byte offsets
into their source text, so one shape serves both languages):

```go
type Severity int        // Error, Warning
type Language int         // Luau, TypeScript

// Spot is one diagnostic anchored to a byte span of the source.
type Spot struct {
    Offset   int          // byte offset of the span start into Source
    Len      int          // span length in bytes; rendered caret count is max(1, …)
    Severity Severity
    Code     string       // optional, shown after message: "TS2322", "noAny", ""
    Message  string
}

type Options struct {
    Context int           // context lines above/below (default 1)
    Color   bool          // caller sets from term.ColorEnabled(w)
}

// Render returns the framed (and, when o.Color, ANSI-colored) block for one
// file's spots. Pure: no I/O, the caller writes the result where it wants.
func Render(path, source string, lang Language, spots []Spot, o Options) string
```

Behavior:

- **Position math.** Line/col are computed from `Offset` by counting newlines in
  `Source` (1-based line, 1-based byte column). No dependency on tsgo's position
  APIs — the same code resolves Luau and TS spots.
- **Layout** (matches the approved mock):

  ```
  error: expected ')'
    --> entry.luau:3:18
     |
   2 | local function f()
   3 |   return print(1 2)
     |                  ^ expected ')'
     |
  ```

  Severity headline (`error:` red / `warning:` yellow), `-->` locator, a gutter
  with right-aligned line numbers, one context line above/below the primary
  line, and a caret/underline line in the severity color.
- **Keyword highlighting.** A word-boundary scan (`[A-Za-z_][A-Za-z0-9_]*`) over
  each visible source line colors words that are in the language's reserved set.
  Both sets live in this package:
  - Luau: the 21 reserved words (`and`/`break`/…/`while`).
  - TypeScript/JS: the standard reserved set (`const`, `let`, `function`,
    `class`, `return`, `if`, `import`, `export`, …).
  Inside the chosen tier this naïve scan may color a keyword that appears inside
  a string/comment on the shown line; that is an accepted cosmetic limitation of
  the keywords-only tier (full tokenization was explicitly declined).
- **Color gating.** The caller passes `o.Color` from `term.ColorEnabled(w)`
  (which honors `NO_COLOR`/`FORCE_COLOR`/TTY). When color is off, the output is
  plain ASCII and byte-stable (so piped output and golden tests are
  deterministic).
- **Edge cases.** Tabs in the source line are expanded to spaces for caret
  alignment; a multi-line span underlines only through the end of the first
  line; `Len == 0` renders a single caret; an out-of-range / empty `Source`
  degrades to the legacy `path:line:col: message` one-liner.

A drift-guard test imports `internal/luau` and asserts diagframe's Luau keyword
set equals the canonical set behind `luau.IsValidIdentifier` (the source of
truth in `internal/luau/validate.go`). To enable it, `internal/luau` exposes a
small `IsReservedKeyword(string) bool` (or an exported set); diagframe keeps its
own copy for rendering but the test pins them together.

### 2. TypeScript / compile plumbing

The renderer needs `(path, source, offset, len)` per diagnostic. Today the
compile layer throws location away. Changes:

- **`DiagnosticInfo`** (`internal/compile/compile.go`) gains `FileName string`,
  `Offset int`, `Len int` (alongside existing `Code`, `Message`, `Warning`).
- **`transformAndRenderDetailed`** populates location from `d.Node`: the token
  start (skipping leading trivia) and end give `Offset`/`Len`; the node's source
  file gives `FileName` and the source text.
- **`tsDiagnosticInfos`** populates location from each `*ast.Diagnostic`
  (`File()`, `Pos()`, `Len()`).
- **Project path** (`internal/compile/project.go`): `compiledProjectSourceFile`
  carries structured per-file diagnostics plus the file's source text. A
  detailed collector surfaces these through `BuildResult`. The existing
  message-only accessors (`diagnosticInfoMessages`, the `[]string`-returning
  `CompileFile`/`CompileProject`/`BuildProjectWithOptions`) are **kept
  unchanged** so conformance/diff tests are untouched; new structured output is
  additive.
- **`cmd/rotor/build.go`**: `runBuildOnce` returns structured diagnostics;
  `buildFailure` (in `cmd/rotor/ui.go`) groups them by file and renders frames
  via `diagframe`. Bonus: the `--json` path (`jsonDiagnostic`) can now emit real
  `file`/`line`/`col` instead of the message-only placeholder it documents
  today.

Source text for the frame is captured at collection time (we hold the
`*ast.SourceFile`), so the renderer never re-reads the disk and virtual/in-memory
test sources work unchanged.

### 3. Luau plumbing

`cmd/rotor/minify.go`, `cmd/rotor/bundle.go` (+ the error surface in
`internal/bundle`), and `internal/pack/luau.go` already hold the source string
and a `[]cst.Diagnostic`. Each maps `cst.Diagnostic` → `diagframe.Spot`
(`Offset` from `Pos.Offset`, `Message`, `Severity=Error`) and calls
`Render(..., diagframe.Luau, …)`. The `internal/bundle` parse-error path
currently returns a formatted `error` string; it will instead return enough
structured context (path, source, diagnostic) for the CLI to frame it, or frame
it at the boundary — chosen in the implementation plan to keep `internal/bundle`
free of presentation concerns.

## Data flow

```
Luau:  source + []cst.Diagnostic ─┐
                                  ├─► []diagframe.Spot ─► diagframe.Render ─► stderr
TS:    *ast.Diagnostic ───────────┤        (per file, with that file's source)
       transformer.Diagnostic ────┘
       (Node → offset/len/source via tsgo AST)
```

## Error handling & degradation

- No color (pipe/redirect/`NO_COLOR`) → plain framed text, ASCII, byte-stable.
- Missing/empty source or out-of-range offset → fall back to
  `path:line:col: message`. The renderer never panics on bad input.
- Warnings render in the warning color and do not change exit codes; existing
  error/warning semantics are unchanged.

## Testing

- **Renderer unit/golden tests** (`internal/diagframe`): basic error, warning,
  multi-spot-per-file, color vs `NO_COLOR`, tab expansion, multi-line span,
  `Len==0`, offset at start/end of file, keyword coloring for both languages.
- **Keyword drift test**: diagframe Luau set == `internal/luau` canonical set.
- **Integration goldens**: updated stderr expectations for `rotor minify`,
  `rotor bundle`, and `rotor build` error cases (these goldens change by design).
- **Regression**: conformance/diff harness still passes (codes/messages and
  generated `.luau` unchanged); `rotor check` output unchanged.

## Files

| File | Change |
|---|---|
| `internal/diagframe/diagframe.go` | new — renderer, `Spot`/`Severity`/`Language`/`Options`, keyword sets |
| `internal/diagframe/diagframe_test.go` | new — golden + edge-case tests |
| `internal/luau/validate.go` | export `IsReservedKeyword` (drift-guard hook) |
| `internal/compile/compile.go` | `DiagnosticInfo` gains location; populate it |
| `internal/compile/project.go` | structured per-file diagnostics + source through `BuildResult` |
| `cmd/rotor/build.go` | render frames; fill `--json` line/col |
| `cmd/rotor/ui.go` | `buildFailure` renders via diagframe |
| `cmd/rotor/minify.go` | frame Luau diagnostics |
| `cmd/rotor/bundle.go` / `internal/bundle/bundle.go` | frame Luau parse errors |
| `internal/pack/luau.go` | frame Luau parse errors |

## Rollout

Single change set (one feature). The renderer lands first (self-contained,
fully tested), then the Luau call sites (smallest, source already in hand), then
the TS plumbing (largest). Roadmap + measured-number upkeep per the standing
convention after completion.
