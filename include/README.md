# include/ — roblox-ts runtime library (vendored verbatim)

`RuntimeLib.lua` and `Promise.lua` are copied byte-for-byte from
`reference/roblox-ts/include/` (roblox-ts 3.0.0). Per the rotor design doc
(docs/superpowers/specs/2026-06-05-rotor-design.md), they are **reused
verbatim — never modified**. Do not edit these files; to update them, re-copy
from the reference checkout.

These are the files `rotor build` writes into a project's include folder
(default `<projectDir>/include`, overridable with `--includePath`, skipped
with `--noInclude`), porting upstream `Project/functions/copyInclude.ts`.

The same bytes are embedded in the rotor binary via `internal/includefiles`
(go:embed cannot reach above its package directory, so that package carries a
second copy); `internal/includefiles/includefiles_test.go` fails if the two
copies ever drift.
