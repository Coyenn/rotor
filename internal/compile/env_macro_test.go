package compile

import (
	"path/filepath"
	"strings"
	"testing"
)

// ----------------------------------------------------------------------------
// $env compile-time environment macro — rotor extension (no rbxtsc
// counterpart; replaces the community rbxts-transform-env plugin).
//
// The fixture deliberately does NOT declare `$env` in its globals.d.ts: the
// type surface must come from the synthetic in-memory __rotor_env.d.ts that
// newProjectProgramFromFS injects (envdecl.go), so this test covers both the
// declaration injection and the transformer inlining (envmacro.go).
//
// Value resolution priority (internal/dotenv): process env >
// .env.<NODE_ENV> > .env, with NODE_ENV itself resolved from the process
// env first, then .env.
// ----------------------------------------------------------------------------

func TestEnvMacroModel(t *testing.T) {
	// MUST be set before compiling: $env inlines at transform time.
	t.Setenv("ROTOR_TEST_ENV_PROC", "from-process") // overrides the .env value "from-file"
	t.Setenv("NODE_ENV", "studio")                  // selects .env.studio (OVERLAY=from-studio)

	files := compileRuntimeLibProject(t, "env_model")

	// In order: .env value; process-env override; fallback arg; unset -> nil;
	// dot access; element access; 2-arity with a set name (fallback unused);
	// .env.<NODE_ENV> override; double/single quote stripping.
	want := "-- Compiled with roblox-ts v3.0.0\n" +
		"print(\"Adventure Quest\")\n" +
		"print(\"from-process\")\n" +
		"print(\"fallback-value\")\n" +
		"print(nil)\n" +
		"print(\"Adventure Quest\")\n" +
		"print(\"Adventure Quest\")\n" +
		"print(\"Adventure Quest\")\n" +
		"print(\"from-studio\")\n" +
		"print(\"hello world\")\n" +
		"print(\"single quoted\")\n" +
		"return nil\n"
	if got := files["out/main.luau"]; got != want {
		t.Errorf("out/main.luau:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

// A non-literal $env argument cannot be inlined at compile time and must
// produce a clear rotor diagnostic — never a panic (which would surface as
// an "internal compiler error").
func TestEnvMacroNonLiteralArgDiagnostic(t *testing.T) {
	text, diags, err := CompileFileDetailed(filepath.Join("testdata", "env_diag_model"), "src/main.ts")
	if err != nil {
		t.Fatalf("CompileFileDetailed returned hard error (want diagnostic): %v", err)
	}
	if text != "" {
		t.Errorf("expected no output text, got:\n%s", text)
	}
	found := false
	for _, d := range diags {
		if d.Code == "rotorEnvNonLiteralArg" {
			found = true
			if !strings.Contains(d.Message, "string literal") {
				t.Errorf("diagnostic message not descriptive: %q", d.Message)
			}
		}
	}
	if !found {
		t.Errorf("diagnostics = %+v, want one with code rotorEnvNonLiteralArg", diags)
	}
}

// A bare `$env` outside call/property/element position (e.g. aliasing it)
// has no runtime value to emit — also a diagnostic, not a panic.
func TestEnvMacroBareUsageDiagnostic(t *testing.T) {
	text, diags, err := CompileFileDetailed(filepath.Join("testdata", "env_diag_model"), "src/bare.ts")
	if err != nil {
		t.Fatalf("CompileFileDetailed returned hard error (want diagnostic): %v", err)
	}
	if text != "" {
		t.Errorf("expected no output text, got:\n%s", text)
	}
	found := false
	for _, d := range diags {
		if d.Code == "rotorEnvBadUsage" {
			found = true
		}
	}
	if !found {
		t.Errorf("diagnostics = %+v, want one with code rotorEnvBadUsage", diags)
	}
}
