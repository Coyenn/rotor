# rotor Release Automation Design

**Date:** 2026-06-08
**Status:** Approved

## Summary

Automate rotor's public binary distribution from GitHub tags. A pushed `vX.Y.Z`
tag should trigger a GitHub Actions workflow that tests rotor, cross-compiles
release binaries for Windows, macOS, and Linux, packages them with predictable
asset names, publishes checksums, and creates a GitHub Release under
`uproot/rotor`.

The release layout should be intentionally generic so GitHub-release-based tool
managers can consume it without bespoke packaging. The first-class targets are
`mise`, `aftman`, and `rokit`, while the asset naming should remain compatible
with direct GitHub release download flows used by `foreman`-style setups.

## Goals

- Publish rotor binaries automatically from `vX.Y.Z` tags.
- Ship native binaries for:
  - Windows amd64
  - Windows arm64
  - Linux amd64
  - Linux arm64
  - macOS amd64
  - macOS arm64
- Keep release asset naming stable and obvious.
- Make the built binary report the tagged version via `rotor --version`.
- Document the release flow and installation story in the README.

## Non-goals

- Separate package-manager-specific publishing pipelines.
- Homebrew, Scoop, Nix, or apt/yum package definitions.
- Detached signing, notarization, or SBOM generation in this pass.

## Architecture

### Build and publish

Use GoReleaser as the release packager and GitHub Actions as the orchestration
layer:

- GitHub Actions triggers on `push.tags: ['v*']`.
- The workflow sets up Go, runs targeted tests, and invokes GoReleaser.
- GoReleaser cross-builds `./cmd/rotor` with `CGO_ENABLED=0`.
- GoReleaser injects the tag version into `main.version` via linker flags.
- GoReleaser archives the binaries and uploads them to the GitHub Release.
- GoReleaser emits a single checksums file alongside the archives.

### Release assets

Release assets must use a stable `rotor-v{{version}}-{{os}}-{{arch}}` pattern:

- `rotor-v1.0.1-windows-amd64.zip`
- `rotor-v1.0.1-windows-arm64.zip`
- `rotor-v1.0.1-linux-amd64.tar.gz`
- `rotor-v1.0.1-linux-arm64.tar.gz`
- `rotor-v1.0.1-darwin-amd64.tar.gz`
- `rotor-v1.0.1-darwin-arm64.tar.gz`
- `rotor-v1.0.1-checksums.txt`

Inside each archive:

- Windows archives contain `rotor.exe`
- Unix archives contain `rotor`
- Archives may also include `README.md` and `LICENSE`

This shape gives the installer tools a conventional GitHub release surface to
download from.

### Versioning

`cmd/rotor/main.go` should stop using a compile-time constant for the release
version and instead use a package-level string variable with a local-build
fallback. Local `go build ./cmd/rotor` should still work and produce a binary
that prints a sensible default version, while release builds should override the
value through `-ldflags -X`.

## Repository changes

- Add `.goreleaser.yml`
- Add `.github/workflows/release.yml`
- Update `cmd/rotor/main.go` for linker-injected versioning
- Update `README.md` with:
  - automated release/tag flow
  - binary download instructions
  - install snippets for `mise`, `aftman`, and `rokit`
  - generic GitHub Release note for foreman-style setups

## Verification

- `go test ./cmd/rotor -count=1`
- Local linker override smoke test:
  - build with `-ldflags "-X main.version=9.9.9"`
  - run the built binary with `--version`
  - expect `9.9.9`
- Validate release config by inspection plus repository tests if GoReleaser is
  unavailable in the workspace

## Risks and mitigations

| Risk | Mitigation |
|---|---|
| Release asset naming does not match installer expectations | Keep names explicit and conventional; document exact filenames and GitHub repo source |
| Cross-platform builds fail due to CGO or platform-specific assumptions | Build rotor with `CGO_ENABLED=0` in GoReleaser |
| README install syntax drifts from current tools | Prefer GitHub-release-based instructions and cite current official docs during implementation |
| GoReleaser binary unavailable locally for validation | Keep config simple, verify YAML carefully, and rely on the GitHub workflow as the final integration check |
