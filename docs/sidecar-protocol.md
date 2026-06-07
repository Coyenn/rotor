# Transformer Sidecar Protocol

This repository now carries a standalone Node sidecar in `tools/sidecar/` for the Phase 4 transformer-plugin slice. It is intentionally separate from Rotor's Go compile path. No Go entrypoint wires it up yet.

## Setup

Install the sidecar-local dependency set:

```bash
cd tools/sidecar
npm install
```

The package pins `typescript@5.5.3` to match the upstream `roblox-ts` plugin runtime.

## Invocation

Run the sidecar as a long-lived stdio worker:

```bash
node tools/sidecar/main.js
```

The process reads newline-delimited JSON messages from `stdin` and writes one newline-delimited JSON response per request to `stdout`.

## Protocol v1

Each request must be a single JSON object with `protocol: 1`.

```json
{
  "protocol": 1,
  "tsConfigPath": "C:/abs/project/tsconfig.json",
  "projectDir": "C:/abs/project",
  "compileFileNames": [
    "C:/abs/project/src/example.ts"
  ],
  "changedFiles": [
    {
      "fileName": "C:/abs/project/src/example.ts",
      "text": "export const phase = \"memory\";\n"
    }
  ]
}
```

Response shape:

```json
{
  "diagnostics": [
    {
      "category": "error",
      "code": "invalid-request",
      "message": "protocol must equal 1"
    }
  ],
  "transformed": [
    {
      "fileName": "C:/abs/project/src/example.ts",
      "text": "export const phase = \"afterDeclarations:before:after:start\";\n"
    }
  ]
}
```

`diagnostics` may contain:

- TypeScript config/program diagnostics converted to `{ category, code, file, start, length, message }`
- transformer resolution warnings using code `transformer-not-found`
- request validation errors using code `invalid-request`
- internal worker failures using code `sidecar-internal`

## Semantics Mirrored From Upstream

The standalone worker mirrors the upstream `roblox-ts` transformer behavior in these areas:

- `getPluginConfigs` re-reads the raw `tsconfig`, keeps child `compilerOptions.plugins` entries before parent `extends` entries, and only accepts plugin objects with string `transform` fields.
- transformer modules resolve relative to `projectDir`.
- factory invocation follows upstream `type` handling for `program`, `config`, `checker`, `raw`, and `compilerOptions`.
- transformed files run through a single `typescript.transformNodes(...)` pass.
- transformer flatten order intentionally stays `after`, then `before`, then `afterDeclarations`.
- transformed `SourceFile`s are reprinted with `typescript.createPrinter().printFile(...)`.

## Warm Session Behavior

The sidecar keeps one in-memory project session per `(projectDir, tsConfigPath)` pair:

- `changedFiles` updates replace file contents in an overlay map and bump script versions.
- the TypeScript `LanguageService` reuses its program across requests when the project identity stays the same.
- source lookup for `compileFileNames` happens against the current overlay-backed program.

## Local Verification

Run the standalone smoke suite:

```bash
cd tools/sidecar
npm test
```

The smoke tests cover:

- plugin discovery through `extends`
- named/default transformer factory loading
- `checker` and `compilerOptions` factory instantiation
- the `after -> before -> afterDeclarations` execution quirk
- stdio protocol handling with warm overlay updates
