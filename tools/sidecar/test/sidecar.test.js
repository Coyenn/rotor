const assert = require("node:assert/strict");
const path = require("node:path");
const { spawn } = require("node:child_process");
const test = require("node:test");
const ts = require("typescript");

const sidecar = require("../index.js");

const projectDir = path.resolve(__dirname, "..", "testdata", "project");
const tsConfigPath = path.join(projectDir, "tsconfig.json");
const sourcePath = path.join(projectDir, "src", "example.ts");
const mainPath = path.resolve(__dirname, "..", "main.js");

function createProgram() {
  const parsed = ts.getParsedCommandLineOfConfigFile(tsConfigPath, {}, ts.sys);
  if (!parsed) {
    throw new Error("expected parsed tsconfig");
  }

  return ts.createProgram({
    rootNames: parsed.fileNames,
    options: parsed.options,
    projectReferences: parsed.projectReferences,
  });
}

function readProtocolLine(stream) {
  return new Promise((resolve, reject) => {
    let buffer = "";

    function cleanup() {
      stream.off("data", onData);
      stream.off("error", onError);
    }

    function onError(error) {
      cleanup();
      reject(error);
    }

    function onData(chunk) {
      buffer += chunk.toString();
      const newlineIndex = buffer.indexOf("\n");
      if (newlineIndex === -1) {
        return;
      }

      const line = buffer.slice(0, newlineIndex);
      cleanup();
      resolve(JSON.parse(line));
    }

    stream.on("data", onData);
    stream.on("error", onError);
  });
}

test("getPluginConfigs keeps child plugins before parent extends plugins", () => {
  const configs = sidecar.getPluginConfigs(ts, tsConfigPath);

  assert.equal(configs.length, 3);
  assert.deepEqual(
    configs.map((config) => ({
      type: config.type ?? "program",
      import: config.import ?? "default",
      prefix: config.prefix,
      after: Boolean(config.after),
      afterDeclarations: Boolean(config.afterDeclarations),
    })),
    [
      { type: "program", import: "default", prefix: "after", after: true, afterDeclarations: false },
      { type: "config", import: "configTransformer", prefix: "before", after: false, afterDeclarations: false },
      { type: "raw", import: "rawTransformer", prefix: "afterDeclarations", after: false, afterDeclarations: true },
    ],
  );
});

test("createTransformerList instantiates checker and compilerOptions factories", () => {
  const program = createProgram();
  const sourceFile = program.getSourceFile(sourcePath);
  assert.ok(sourceFile, "expected source file");

  const { transforms, diagnostics } = sidecar.createTransformerList(ts, program, [
    {
      transform: "./plugins/prefix-string-named.js",
      import: "compilerOptionsTransformer",
      type: "compilerOptions",
      prefix: "options",
      after: true,
    },
    {
      transform: "./plugins/prefix-string-named.js",
      import: "checkerTransformer",
      type: "checker",
      prefix: "checker",
    },
  ], projectDir);

  assert.deepEqual(diagnostics, []);

  const result = sidecar.transformSourceFiles(ts, program, [sourceFile], transforms);
  assert.deepEqual(
    result.diagnostics,
    [],
  );
  assert.match(result.transformed[0].text, /checker:options:start/);
});

test("main.js serves protocol v1 requests and reuses overlay updates", async () => {
  const child = spawn(process.execPath, [mainPath], {
    cwd: path.resolve(__dirname, ".."),
    stdio: ["pipe", "pipe", "pipe"],
  });

  const stderr = [];
  child.stderr.on("data", (chunk) => {
    stderr.push(chunk.toString());
  });

  try {
    const firstResponsePromise = readProtocolLine(child.stdout);
    child.stdin.write(`${JSON.stringify({
      protocol: 1,
      projectDir,
      tsConfigPath,
      compileFileNames: [sourcePath],
      changedFiles: [],
    })}\n`);

    const firstResponse = await firstResponsePromise;
    assert.deepEqual(firstResponse.diagnostics, []);
    assert.equal(firstResponse.transformed.length, 1);
    assert.match(firstResponse.transformed[0].text, /afterDeclarations:before:after:start/);

    const secondResponsePromise = readProtocolLine(child.stdout);
    child.stdin.write(`${JSON.stringify({
      protocol: 1,
      projectDir,
      tsConfigPath,
      compileFileNames: [sourcePath],
      changedFiles: [
        {
          fileName: sourcePath,
          text: 'export const phase = "memory";\n',
        },
      ],
    })}\n`);

    const secondResponse = await secondResponsePromise;
    assert.deepEqual(secondResponse.diagnostics, []);
    assert.equal(secondResponse.transformed.length, 1);
    assert.match(secondResponse.transformed[0].text, /afterDeclarations:before:after:memory/);
  } finally {
    child.stdin.end();
    await new Promise((resolve) => child.once("exit", resolve));
  }

  assert.deepEqual(stderr, []);
});
