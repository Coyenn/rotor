const ts = require("typescript");

function createPrefixTransformer(tsApi, prefix) {
  return (context) => {
    const visit = (node) => {
      if (tsApi.isStringLiteral(node)) {
        return tsApi.factory.createStringLiteral(`${prefix}:${node.text}`);
      }

      return tsApi.visitEachChild(node, visit, context);
    };

    return (sourceFile) => tsApi.visitNode(sourceFile, visit);
  };
}

module.exports = function programTransformer(program, config, helpers) {
  if (!program.getTypeChecker()) {
    throw new Error("missing program checker");
  }
  if (!helpers || helpers.ts !== ts) {
    throw new Error("missing ts helper");
  }

  return createPrefixTransformer(helpers.ts, config.prefix);
};

module.exports.configTransformer = function configTransformer(config) {
  if ("type" in config || "after" in config || "afterDeclarations" in config) {
    throw new Error("unexpected control flags leaked into config transformer");
  }

  return createPrefixTransformer(ts, config.prefix);
};

module.exports.rawTransformer = function rawTransformer(context, program, config) {
  if (!program.getTypeChecker()) {
    throw new Error("missing raw program checker");
  }

  const visit = (node) => {
    if (ts.isStringLiteral(node)) {
      return ts.factory.createStringLiteral(`${config.prefix}:${node.text}`);
    }

    return ts.visitEachChild(node, visit, context);
  };

  return (sourceFile) => ts.visitNode(sourceFile, visit);
};

module.exports.checkerTransformer = function checkerTransformer(checker, config) {
  if (typeof checker.getProgram !== "function") {
    throw new Error("missing checker program");
  }

  return createPrefixTransformer(ts, config.prefix);
};

module.exports.compilerOptionsTransformer = function compilerOptionsTransformer(options, config) {
  if (!options.module) {
    throw new Error("missing compiler options");
  }

  return createPrefixTransformer(ts, config.prefix);
};
