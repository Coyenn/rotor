const ts = require("typescript");

function createPrefixTransformer(prefix) {
  return (context) => {
    const visit = (node) => {
      if (ts.isStringLiteral(node)) {
        return ts.factory.createStringLiteral(`${prefix}:${node.text}`);
      }

      return ts.visitEachChild(node, visit, context);
    };

    return (sourceFile) => ts.visitNode(sourceFile, visit);
  };
}

function assertNoControlFlags(config) {
  if ("type" in config || "after" in config || "afterDeclarations" in config) {
    throw new Error("unexpected control flags leaked into config transformer");
  }
}

module.exports = {
  configTransformer(config) {
    assertNoControlFlags(config);
    return createPrefixTransformer(config.prefix);
  },

  rawTransformer(context, program, config) {
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
  },

  checkerTransformer(checker, config) {
    if (typeof checker.getTypeAtLocation !== "function") {
      throw new Error("missing checker access");
    }

    return createPrefixTransformer(config.prefix);
  },

  compilerOptionsTransformer(options, config) {
    if (!options.module) {
      throw new Error("missing compiler options");
    }

    return createPrefixTransformer(config.prefix);
  },
};
