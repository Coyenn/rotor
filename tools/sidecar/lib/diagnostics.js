const ts = require("typescript");

function categoryToString(category) {
  switch (category) {
    case ts.DiagnosticCategory.Warning:
      return "warning";
    case ts.DiagnosticCategory.Message:
    case ts.DiagnosticCategory.Suggestion:
      return "warning";
    case ts.DiagnosticCategory.Error:
    default:
      return "error";
  }
}

function toProtocolDiagnostic(diagnostic) {
  return {
    category: categoryToString(diagnostic.category),
    code: String(diagnostic.code),
    file: diagnostic.file ? diagnostic.file.fileName : undefined,
    start: diagnostic.start,
    length: diagnostic.length,
    message: ts.flattenDiagnosticMessageText(diagnostic.messageText, "\n"),
  };
}

function createProtocolDiagnostic(category, code, message, file, start, length) {
  return {
    category,
    code,
    file,
    start,
    length,
    message,
  };
}

function createPluginNotFoundDiagnostic(transformName, error) {
  const reason = error instanceof Error ? error.message : String(error);
  return createProtocolDiagnostic(
    "warning",
    "transformer-not-found",
    `Transformer \`${transformName}\` was not found!\nMore info: ${reason}\nDid you forget to install the package?`,
  );
}

function createRequestDiagnostic(message) {
  return createProtocolDiagnostic("error", "invalid-request", message);
}

function createInternalDiagnostic(error) {
  const message = error instanceof Error ? error.stack ?? error.message : String(error);
  return createProtocolDiagnostic("error", "sidecar-internal", message);
}

module.exports = {
  createInternalDiagnostic,
  createPluginNotFoundDiagnostic,
  createProtocolDiagnostic,
  createRequestDiagnostic,
  toProtocolDiagnostic,
};
