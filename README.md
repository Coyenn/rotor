# rotor

Native-speed drop-in replacement for the roblox-ts compiler (`rbxtsc`), written in Go
on top of [typescript-go](https://github.com/microsoft/typescript-go).

Goal: 1:1 output compatibility with roblox-ts 3.0.0 — same `@rbxts/*` packages, same
CLI, byte-identical Luau output — at ~10x the speed.

See `docs/superpowers/specs/2026-06-05-rotor-design.md` for the design.

## Layout
- `tsgo/` — generated mirror of typescript-go internals (do not edit; run `go run ./tools/mirror`)
- `reference/` — pinned roblox-ts sources we port from and diff against
- `internal/luau` — Luau AST + renderer (port of @roblox-ts/luau-ast)
