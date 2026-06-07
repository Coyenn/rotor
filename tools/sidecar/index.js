const readline = require("node:readline");
const ts = require("typescript");
const { createInternalDiagnostic, createRequestDiagnostic } = require("./lib/diagnostics");
const { createTransformerList, flattenIntoTransformers, getPluginConfigs } = require("./lib/plugins");
const { SidecarProjectSession, SidecarServer } = require("./lib/session");

function transformSourceFiles(tsApi, program, sourceFiles, transforms) {
  const session = new SidecarProjectSession(tsApi, process.cwd(), process.cwd());
  return session.transformSourceFiles(program, sourceFiles, transforms);
}

function serveStdio(options = {}) {
  const input = options.input ?? process.stdin;
  const output = options.output ?? process.stdout;
  const server = options.server ?? new SidecarServer(options.ts ?? ts);
  const lineReader = readline.createInterface({
    input,
    crlfDelay: Infinity,
  });

  lineReader.on("line", (line) => {
    if (!line.trim()) {
      return;
    }

    let response;
    try {
      const request = JSON.parse(line);
      response = server.handleRequest(request);
    } catch (error) {
      response = {
        diagnostics: [
          error instanceof SyntaxError
            ? createRequestDiagnostic(`invalid JSON request: ${error.message}`)
            : createInternalDiagnostic(error),
        ],
        transformed: [],
      };
    }

    output.write(`${JSON.stringify(response)}\n`);
  });

  return lineReader;
}

module.exports = {
  SidecarProjectSession,
  SidecarServer,
  createTransformerList,
  flattenIntoTransformers,
  getPluginConfigs,
  serveStdio,
  transformSourceFiles,
};
