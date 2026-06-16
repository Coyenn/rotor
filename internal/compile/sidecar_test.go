package compile

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"rotor/internal/logservice"
)

// repoSidecarDir returns tools/sidecar in this repo checkout. Synthetic
// plugin fixtures have no typescript of their own, so tests point
// ROTOR_SIDECAR_PATH here and the worker falls back to the sidecar's
// pinned typescript install.
func repoSidecarDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Join(filepath.Dir(file), "..", "..", "tools", "sidecar")
	if _, err := os.Stat(filepath.Join(dir, "main.js")); err != nil {
		t.Fatalf("repo sidecar missing: %v", err)
	}
	return filepath.Clean(dir)
}

func setRepoSidecarPath(t *testing.T) {
	t.Helper()
	t.Setenv("ROTOR_SIDECAR_PATH", repoSidecarDir(t))
}

const prefixStringPlugin = `const ts = require("typescript");

module.exports = function programTransformer(program, config, helpers) {
	if (!program.getTypeChecker()) {
		throw new Error("missing program checker");
	}
	if (!helpers || helpers.ts !== ts) {
		throw new Error("missing ts helper");
	}

	return (context) => {
		const visit = (node) => {
			if (ts.isStringLiteral(node)) {
				return ts.factory.createStringLiteral(config.prefix + ":" + node.text);
			}
			return ts.visitEachChild(node, visit, context);
		};
		return (sourceFile) => ts.visitNode(sourceFile, visit);
	};
};
`

func TestBuildProjectTransformerPluginSidecar(t *testing.T) {
	setRepoSidecarPath(t)
	closeSidecarSessions()
	dir := writeProject(t, "@scope/plugin-fixture", "")
	// Registered after writeProject's t.TempDir so it runs before the temp
	// dir is removed (the worker's cwd is the project dir).
	t.Cleanup(closeSidecarSessions)
	writeSidecarPluginFixture(t, dir, `{
	"compilerOptions": {
		"allowSyntheticDefaultImports": true,
		"module": "CommonJS",
		"moduleResolution": "Node",
		"noLib": true,
		"moduleDetection": "force",
		"strict": true,
		"target": "ESNext",
		"types": [],
		"typeRoots": ["node_modules/@rbxts"],
		"rootDir": "src",
		"outDir": "out",
		"plugins": [
			{
				"transform": "./plugins/prefix-string.js",
				"prefix": "plugin"
			}
		]
	}
}`, `{
	"extends": "./tsconfig.base.json",
	"compilerOptions": {
		"allowSyntheticDefaultImports": true,
		"module": "CommonJS",
		"moduleResolution": "Node",
		"noLib": true,
		"moduleDetection": "force",
		"strict": true,
		"target": "ESNext",
		"types": [],
		"typeRoots": ["node_modules/@rbxts"],
		"rootDir": "src",
		"outDir": "out"
	},
	"include": ["src"]
}`)

	if err := os.WriteFile(filepath.Join(dir, "src", "main.ts"), []byte("export const phase = \"start\";\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, diags, err := BuildProjectWithOptions(dir, ProjectOptions{})
	if err != nil {
		t.Fatalf("BuildProjectWithOptions: %v (diags: %v)", err, diags)
	}
	if len(diags) > 0 {
		t.Fatalf("diagnostics: %v", diags)
	}
	if result == nil {
		t.Fatal("nil result")
	}
	got := result.Outputs["out/main.luau"]
	if !strings.Contains(got, `local phase = "plugin:start"`) {
		t.Fatalf("out/main.luau missing transformed string:\n%s", got)
	}
}

func TestBuildProjectWithoutPluginsDoesNotRequireNode(t *testing.T) {
	dir := writeProject(t, "@scope/no-plugin-fixture", "")
	t.Setenv("ROTOR_NODE_PATH", filepath.Join(dir, "missing-node"))

	result, diags, err := BuildProjectWithOptions(dir, ProjectOptions{})
	if err != nil {
		t.Fatalf("BuildProjectWithOptions: %v (diags: %v)", err, diags)
	}
	if len(diags) > 0 {
		t.Fatalf("diagnostics: %v", diags)
	}
	if result == nil {
		t.Fatal("nil result")
	}
}

func TestBuildProjectTransformerPluginRequiresNode(t *testing.T) {
	setRepoSidecarPath(t)
	dir := writeProject(t, "@scope/plugin-node-fixture", "")
	writeSidecarPluginFixture(t, dir, "", `{
	"compilerOptions": {
		"allowSyntheticDefaultImports": true,
		"module": "CommonJS",
		"moduleResolution": "Node",
		"noLib": true,
		"moduleDetection": "force",
		"strict": true,
		"target": "ESNext",
		"types": [],
		"typeRoots": ["node_modules/@rbxts"],
		"rootDir": "src",
		"outDir": "out",
		"plugins": [
			{
				"transform": "./plugins/prefix-string.js",
				"prefix": "plugin"
			}
		]
	},
	"include": ["src"]
}`)
	t.Setenv("ROTOR_NODE_PATH", filepath.Join(dir, "missing-node"))

	_, diags, err := BuildProjectWithOptions(dir, ProjectOptions{})
	if err == nil {
		t.Fatal("expected missing-node error")
	}
	if len(diags) != 1 || !strings.Contains(diags[0], "node executable not found") {
		t.Fatalf("diags = %v, want missing-node diagnostic", diags)
	}
}

func TestBuildProjectMissingTransformerWarnsAndContinues(t *testing.T) {
	setRepoSidecarPath(t)
	closeSidecarSessions()
	dir := writeProject(t, "@scope/plugin-warning-fixture", "")
	t.Cleanup(closeSidecarSessions)
	tsconfig := `{
	"compilerOptions": {
		"allowSyntheticDefaultImports": true,
		"module": "CommonJS",
		"moduleResolution": "Node",
		"noLib": true,
		"moduleDetection": "force",
		"strict": true,
		"target": "ESNext",
		"types": [],
		"typeRoots": ["node_modules/@rbxts"],
		"rootDir": "src",
		"outDir": "out",
		"plugins": [
			{
				"transform": "./plugins/does-not-exist.js",
				"prefix": "plugin"
			}
		]
	},
	"include": ["src"]
}`
	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(tsconfig), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "src", "main.ts"), []byte("export const phase = \"start\";\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var warnings bytes.Buffer
	oldOutput := logservice.Output
	oldVerbose := logservice.Verbose
	logservice.Output = &warnings
	logservice.Verbose = false
	t.Cleanup(func() {
		logservice.Output = oldOutput
		logservice.Verbose = oldVerbose
	})

	result, diags, err := BuildProjectWithOptions(dir, ProjectOptions{})
	if err != nil {
		t.Fatalf("BuildProjectWithOptions: %v (diags: %v)", err, diags)
	}
	if len(diags) > 0 {
		t.Fatalf("diagnostics: %v", diags)
	}
	if result == nil {
		t.Fatal("nil result")
	}
	got := result.Outputs["out/main.luau"]
	if !strings.Contains(got, `local phase = "start"`) {
		t.Fatalf("out/main.luau should keep original string when plugin is missing:\n%s", got)
	}
	logText := warnings.String()
	if !strings.Contains(logText, "Compiler Warning:") || !strings.Contains(logText, "Transformer `./plugins/does-not-exist.js` was not found!") {
		t.Fatalf("warning output = %q, want transformer warning", logText)
	}
}

const countingPlugin = `let buildCount = 0;

module.exports = function (program, config, helpers) {
	const ts = helpers.ts;
	buildCount += 1;
	return (context) => (sourceFile) => {
		const visit = (node) => {
			if (ts.isStringLiteral(node) && node.text === "BUILD_COUNT") {
				return ts.factory.createStringLiteral("build:" + buildCount);
			}
			return ts.visitEachChild(node, visit, context);
		};
		return ts.visitNode(sourceFile, visit);
	};
};
`

func TestBuildProjectTransformerSidecarStaysWarmAcrossBuilds(t *testing.T) {
	setRepoSidecarPath(t)
	closeSidecarSessions()

	dir := writeProject(t, "@scope/plugin-warm-fixture", "")
	t.Cleanup(closeSidecarSessions)
	if err := os.MkdirAll(filepath.Join(dir, "plugins"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugins", "counting.js"), []byte(countingPlugin), 0o644); err != nil {
		t.Fatal(err)
	}
	tsconfig := `{
	"compilerOptions": {
		"allowSyntheticDefaultImports": true,
		"module": "CommonJS",
		"moduleResolution": "Node",
		"noLib": true,
		"moduleDetection": "force",
		"strict": true,
		"target": "ESNext",
		"types": [],
		"typeRoots": ["node_modules/@rbxts"],
		"rootDir": "src",
		"outDir": "out",
		"plugins": [{ "transform": "./plugins/counting.js" }]
	},
	"include": ["src"]
}`
	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(tsconfig), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "src", "main.ts"), []byte("export const tag = \"BUILD_COUNT\";\nexport const phase = \"first\";\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, diags, err := BuildProjectWithOptions(dir, ProjectOptions{})
	if err != nil {
		t.Fatalf("first build: %v (diags: %v)", err, diags)
	}
	got := result.Outputs["out/main.luau"]
	if !strings.Contains(got, `local tag = "build:1"`) || !strings.Contains(got, `local phase = "first"`) {
		t.Fatalf("first build output unexpected:\n%s", got)
	}

	if err := os.WriteFile(filepath.Join(dir, "src", "main.ts"), []byte("export const tag = \"BUILD_COUNT\";\nexport const phase = \"second\";\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, diags, err = BuildProjectWithOptions(dir, ProjectOptions{})
	if err != nil {
		t.Fatalf("second build: %v (diags: %v)", err, diags)
	}
	got = result.Outputs["out/main.luau"]
	if !strings.Contains(got, `local tag = "build:2"`) {
		t.Fatalf("second build did not reuse a warm sidecar process:\n%s", got)
	}
	if !strings.Contains(got, `local phase = "second"`) {
		t.Fatalf("warm sidecar served a stale snapshot (changedFiles overlay broken):\n%s", got)
	}
}

// getterRenamePlugin is a minimal stand-in for rbxts-transformer-setget: it
// renames every `get x()` accessor to a `__getx()` method and rewrites `.x`
// reads (resolved through the type checker) into `.__getx()` calls. The rewrite
// only type-checks when the DECLARATION file was transformed too, so it
// exercises the cross-file overlay-coherence invariant.
const getterRenamePlugin = `module.exports = function (program, config, helpers) {
	const ts = helpers.ts;
	const checker = program.getTypeChecker();
	return (context) => {
		const visit = (node) => {
			if (ts.isGetAccessorDeclaration(node) && ts.isIdentifier(node.name)) {
				return ts.factory.createMethodDeclaration(
					node.modifiers,
					undefined,
					ts.factory.createIdentifier("__get" + node.name.text),
					undefined,
					node.typeParameters,
					node.parameters,
					node.type,
					node.body,
				);
			}
			if (ts.isPropertyAccessExpression(node) && ts.isIdentifier(node.name)) {
				const symbol = checker.getSymbolAtLocation(node);
				const declarations = symbol ? symbol.declarations || [] : [];
				if (declarations.some((declaration) => ts.isGetAccessorDeclaration(declaration))) {
					return ts.factory.createCallExpression(
						ts.factory.createPropertyAccessExpression(
							ts.visitNode(node.expression, visit),
							ts.factory.createIdentifier("__get" + node.name.text),
						),
						undefined,
						[],
					);
				}
			}
			return ts.visitEachChild(node, visit, context);
		};
		return (sourceFile) => ts.visitNode(sourceFile, visit);
	};
};
`

// TestBuildProjectTransformerSidecarIncrementalTransformsAllSources is the
// regression test for the `Property '__getx' does not exist` failures: an
// incremental rebuild that recompiles only a changed call-site file must still
// transform the WHOLE project, or the unchanged declaration file is left
// untransformed in the overlay program and the rewritten call site fails to
// type-check.
func TestBuildProjectTransformerSidecarIncrementalTransformsAllSources(t *testing.T) {
	setRepoSidecarPath(t)
	closeSidecarSessions()

	dir := writeProject(t, "@scope/plugin-incremental-fixture", "")
	t.Cleanup(closeSidecarSessions)
	if err := os.MkdirAll(filepath.Join(dir, "plugins"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugins", "getter.js"), []byte(getterRenamePlugin), 0o644); err != nil {
		t.Fatal(err)
	}
	tsconfig := `{
	"compilerOptions": {
		"allowSyntheticDefaultImports": true,
		"module": "CommonJS",
		"moduleResolution": "Node",
		"noLib": true,
		"moduleDetection": "force",
		"strict": true,
		"target": "ESNext",
		"types": [],
		"typeRoots": ["node_modules/@rbxts"],
		"rootDir": "src",
		"outDir": "out",
		"incremental": true,
		"tsBuildInfoFile": "out/tsconfig.tsbuildinfo",
		"plugins": [{ "transform": "./plugins/getter.js" }]
	},
	"include": ["src"]
}`
	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(tsconfig), 0o644); err != nil {
		t.Fatal(err)
	}
	model := "export class Model {\n\tprivate innerValue = 1;\n\tpublic get value(): number {\n\t\treturn this.innerValue;\n\t}\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "src", "model.ts"), []byte(model), 0o644); err != nil {
		t.Fatal(err)
	}
	writeUsage := func(suffix string) {
		t.Helper()
		usage := "import { Model } from \"./model\";\n\nexport function readValue(model: Model): number {\n\treturn model.value;\n}\n" + suffix
		if err := os.WriteFile(filepath.Join(dir, "src", "usage.ts"), []byte(usage), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeUsage("")
	// Drop writeProject's placeholder so only model.ts + usage.ts compile.
	if err := os.Remove(filepath.Join(dir, "src", "main.ts")); err != nil {
		t.Fatal(err)
	}

	result, diags, err := BuildProjectWithOptions(dir, ProjectOptions{})
	if err != nil {
		t.Fatalf("first build: %v (diags: %v)", err, diags)
	}
	if got := result.Outputs["out/usage.luau"]; !strings.Contains(got, "__getvalue") {
		t.Fatalf("first build did not rewrite the getter call site:\n%s", got)
	}

	// Edit ONLY the call-site file. Incremental selection compiles just
	// usage.ts; model.ts (the getter declaration) is unchanged, so the overlay
	// must still transform it or `.__getvalue()` will not resolve.
	writeUsage("export const tag = \"edited\";\n")

	result, diags, err = BuildProjectWithOptions(dir, ProjectOptions{})
	if err != nil {
		t.Fatalf("incremental build after editing only the call site: %v (diags: %v)", err, diags)
	}
	if got := result.Outputs["out/usage.luau"]; !strings.Contains(got, "__getvalue") {
		t.Fatalf("incremental build lost the getter rewrite:\n%s", got)
	}
}

func writeSidecarPluginFixture(t *testing.T, dir, baseTSConfig, rootTSConfig string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "plugins"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugins", "prefix-string.js"), []byte(prefixStringPlugin), 0o644); err != nil {
		t.Fatal(err)
	}
	if baseTSConfig != "" {
		if err := os.WriteFile(filepath.Join(dir, "tsconfig.base.json"), []byte(baseTSConfig), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if rootTSConfig != "" {
		if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(rootTSConfig), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
