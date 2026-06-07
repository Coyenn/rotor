package compile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"rotor/internal/transformer"
)

func TestCompileProjectCommentDirectivesRejectedByDefault(t *testing.T) {
	dir := writeProject(t, "@scope/comment-directives", "")
	src := strings.Join([]string{
		"// @ts-ignore",
		"// @ts-expect-error",
		"// @ts-nocheck",
		"export const value = 1;",
		"",
	}, "\n")
	if err := os.WriteFile(filepath.Join(dir, "src", "main.ts"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	files, diags, err := CompileProject(dir)
	if err == nil {
		t.Fatal("expected comment-directive diagnostics")
	}
	if files != nil {
		t.Fatalf("files = %v, want nil", files)
	}
	if len(diags) != 3 {
		t.Fatalf("len(diags) = %d, want 3 (%v)", len(diags), diags)
	}
	for _, diag := range diags {
		if diag != transformer.DiagNoCommentDirectives(nil).Message {
			t.Fatalf("diag = %q, want %q", diag, transformer.DiagNoCommentDirectives(nil).Message)
		}
	}
}

func TestCompileProjectAllowCommentDirectivesSuppressesDiagnostics(t *testing.T) {
	dir := writeProject(t, "@scope/comment-directives-allowed", "")
	src := strings.Join([]string{
		"// @ts-ignore",
		"// @ts-expect-error",
		"// @ts-nocheck",
		"export const value = 1;",
		"",
	}, "\n")
	if err := os.WriteFile(filepath.Join(dir, "src", "main.ts"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	files, diags, err := CompileProjectWithOptions(dir, ProjectOptions{AllowCommentDirectives: true})
	if err != nil {
		t.Fatalf("CompileProjectWithOptions: %v (diags: %v)", err, diags)
	}
	if len(diags) > 0 {
		t.Fatalf("diagnostics: %v", diags)
	}
	if _, ok := files["out/main.luau"]; !ok {
		t.Fatalf("out/main.luau missing (%v)", keys(files))
	}
}
