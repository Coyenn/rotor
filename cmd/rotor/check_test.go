package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// writeCheckableProject writes a minimal project `rotor check` can typecheck
// without node_modules (noLib + local global stubs). mainSrc overrides
// src/main.ts when non-empty.
func writeCheckableProject(t *testing.T, mainSrc string) string {
	t.Helper()
	dir := t.TempDir()
	tsconfig := `{
	"compilerOptions": {
		"noLib": true,
		"strict": true,
		"target": "ESNext",
		"types": [],
		"typeRoots": ["node_modules/@rbxts"],
		"rootDir": "src",
		"outDir": "out"
	},
	"include": ["src"]
}`
	mustWrite(t, filepath.Join(dir, "tsconfig.json"), tsconfig)
	mustWrite(t, filepath.Join(dir, "src", "globals.d.ts"), noLibGlobalStubs)
	if mainSrc == "" {
		mainSrc = "export {};\n"
	}
	mustWrite(t, filepath.Join(dir, "src", "main.ts"), mainSrc)
	return dir
}

func TestCmdCheckJSONClean(t *testing.T) {
	dir := writeCheckableProject(t, "")

	output, code := captureStdout(t, func() int {
		return cmdCheck([]string{"--json", dir})
	})
	if code != 0 {
		t.Fatalf("cmdCheck --json (clean) exit = %d, want 0; output:\n%s", code, output)
	}

	var res jsonResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &res); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput:\n%s", err, output)
	}
	if !res.OK {
		t.Errorf("ok = false on a clean check; diags: %+v", res.Diagnostics)
	}
	if res.Files <= 0 {
		t.Errorf("files = %d, want > 0", res.Files)
	}
	if res.Diagnostics == nil {
		t.Error("diagnostics must be [] not null")
	}
}

func TestCmdCheckJSONWithDiagnostic(t *testing.T) {
	dir := writeCheckableProject(t, "export const s: string = 5;\n")

	output, code := captureStdout(t, func() int {
		return cmdCheck([]string{"--json", dir})
	})
	if code != 1 {
		t.Fatalf("cmdCheck --json (error) exit = %d, want 1; output:\n%s", code, output)
	}

	var res jsonResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &res); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput:\n%s", err, output)
	}
	if res.OK {
		t.Error("ok = true on a check with errors")
	}
	if len(res.Diagnostics) == 0 {
		t.Fatal("expected at least one diagnostic")
	}
	d := res.Diagnostics[0]
	if d.Severity != "error" {
		t.Errorf("severity = %q, want error", d.Severity)
	}
	// check has structured locations: line/col are 1-based, file is set.
	if d.Line < 1 || d.Col < 1 {
		t.Errorf("line/col = %d/%d, want >= 1/1", d.Line, d.Col)
	}
	if !strings.Contains(filepath.ToSlash(d.File), "main.ts") {
		t.Errorf("file = %q, want it to reference main.ts", d.File)
	}
	if d.Message == "" {
		t.Error("diagnostic message is empty")
	}
}
