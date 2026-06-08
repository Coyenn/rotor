package compile

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"rotor/internal/logservice"
)

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
	dir := writeProject(t, "@scope/plugin-fixture", "")
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
	dir := writeProject(t, "@scope/plugin-warning-fixture", "")
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
