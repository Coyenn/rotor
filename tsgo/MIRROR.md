# Mirror of microsoft/typescript-go internals

- Source: https://github.com/microsoft/typescript-go
- Commit: 1f955e97ae01ad4f47b6a3b0bf5942b330ae6ef3
- Vendored: 2026-06-06T04:35:47Z
- Changes: files copied from internal/ with import paths rewritten
  ("github.com/microsoft/typescript-go/internal/" -> "rotor/tsgo/"); *_test.go files and testdata/ directories omitted.
  No other modifications to mirrored files. Regenerate with:
  go run ./tools/mirror
- Rotor additions (NOT from the mirror): overlay shims from
  tools/mirror/overlay (e.g. checker/rotor_exports.go) are applied
  automatically by tools/mirror after regenerating.
