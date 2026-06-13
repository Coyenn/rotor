package compile

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeFile writes content to a path, creating parent dirs.
func writeGitFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestGitStampProviderLooseRefs builds a fake .git with a loose branch ref and a
// loose tag pointing at the same sha, and asserts the native reader resolves the
// short sha, branch, and tag.
func TestGitStampProviderLooseRefs(t *testing.T) {
	dir := t.TempDir()
	gitDir := filepath.Join(dir, ".git")
	const fullSHA = "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0"

	writeGitFile(t, filepath.Join(gitDir, "HEAD"), "ref: refs/heads/main\n")
	writeGitFile(t, filepath.Join(gitDir, "refs", "heads", "main"), fullSHA+"\n")
	writeGitFile(t, filepath.Join(gitDir, "refs", "tags", "v1.0.0"), fullSHA+"\n")

	p := newStampProvider(filepath.ToSlash(dir), time.Date(2026, 6, 13, 9, 41, 5, 0, time.UTC))

	if got := p.GitSHA(); got != "a1b2c3d" {
		t.Errorf("GitSHA() = %q, want %q", got, "a1b2c3d")
	}
	if got := p.GitBranch(); got != "main" {
		t.Errorf("GitBranch() = %q, want %q", got, "main")
	}
	if got := p.GitTag(); got != "v1.0.0" {
		t.Errorf("GitTag() = %q, want %q", got, "v1.0.0")
	}
	if got := p.BuildTime(); got != "2026-06-13T09:41:05Z" {
		t.Errorf("BuildTime() = %q, want ISO-8601 of the fixed time", got)
	}
}

// TestGitStampProviderPackedRefs resolves a branch and a lightweight tag from
// packed-refs (no loose ref files).
func TestGitStampProviderPackedRefs(t *testing.T) {
	dir := t.TempDir()
	gitDir := filepath.Join(dir, ".git")
	const fullSHA = "0123456789abcdef0123456789abcdef01234567"

	writeGitFile(t, filepath.Join(gitDir, "HEAD"), "ref: refs/heads/release\n")
	writeGitFile(t, filepath.Join(gitDir, "packed-refs"),
		"# pack-refs with: peeled fully-peeled sorted\n"+
			fullSHA+" refs/heads/release\n"+
			fullSHA+" refs/tags/v2.3.4\n")

	p := newStampProvider(filepath.ToSlash(dir), time.Now())

	if got := p.GitSHA(); got != "0123456" {
		t.Errorf("GitSHA() = %q, want %q", got, "0123456")
	}
	if got := p.GitBranch(); got != "release" {
		t.Errorf("GitBranch() = %q, want %q", got, "release")
	}
	if got := p.GitTag(); got != "v2.3.4" {
		t.Errorf("GitTag() = %q, want %q", got, "v2.3.4")
	}
}

// TestGitStampProviderAnnotatedTagPeel matches an annotated tag via its peel
// line (^<commit-sha>), even though the tag's own object sha differs.
func TestGitStampProviderAnnotatedTagPeel(t *testing.T) {
	dir := t.TempDir()
	gitDir := filepath.Join(dir, ".git")
	const commitSHA = "aaaaaaaabbbbbbbbccccccccddddddddeeeeeeee"
	const tagObjSHA = "1111111122222222333333334444444455555555"

	writeGitFile(t, filepath.Join(gitDir, "HEAD"), commitSHA+"\n") // detached HEAD
	writeGitFile(t, filepath.Join(gitDir, "packed-refs"),
		"# pack-refs with: peeled fully-peeled sorted\n"+
			tagObjSHA+" refs/tags/v3.0.0\n"+
			"^"+commitSHA+"\n")

	p := newStampProvider(filepath.ToSlash(dir), time.Now())

	if got := p.GitSHA(); got != "aaaaaaa" {
		t.Errorf("GitSHA() = %q, want %q", got, "aaaaaaa")
	}
	if got := p.GitBranch(); got != "" {
		t.Errorf("GitBranch() = %q, want \"\" for detached HEAD", got)
	}
	if got := p.GitTag(); got != "v3.0.0" {
		t.Errorf("GitTag() = %q, want %q (annotated tag via peel line)", got, "v3.0.0")
	}
}

// TestGitStampProviderNotARepo: outside any git repo every field is empty/false
// and never an error.
func TestGitStampProviderNotARepo(t *testing.T) {
	dir := t.TempDir()
	p := newStampProvider(filepath.ToSlash(dir), time.Now())

	if got := p.GitSHA(); got != "" {
		t.Errorf("GitSHA() = %q, want \"\"", got)
	}
	if got := p.GitBranch(); got != "" {
		t.Errorf("GitBranch() = %q, want \"\"", got)
	}
	if got := p.GitTag(); got != "" {
		t.Errorf("GitTag() = %q, want \"\"", got)
	}
	if p.GitDirty() {
		t.Error("GitDirty() = true, want false outside a repo")
	}
}

// TestGitDirObjectStatePicked: a `.git` FILE redirect (worktree form) resolves
// its gitdir, and HEAD is read from the redirected path.
func TestGitDirFileRedirect(t *testing.T) {
	dir := t.TempDir()
	realGit := filepath.Join(dir, "actual-git")
	const fullSHA = "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"

	writeGitFile(t, filepath.Join(realGit, "HEAD"), "ref: refs/heads/wt\n")
	writeGitFile(t, filepath.Join(realGit, "refs", "heads", "wt"), fullSHA+"\n")
	// `.git` file in the project dir redirects to realGit (absolute path).
	writeGitFile(t, filepath.Join(dir, ".git"), "gitdir: "+realGit+"\n")

	p := newStampProvider(filepath.ToSlash(dir), time.Now())
	if got := p.GitSHA(); got != "deadbee" {
		t.Errorf("GitSHA() = %q, want %q via .git redirect", got, "deadbee")
	}
	if got := p.GitBranch(); got != "wt" {
		t.Errorf("GitBranch() = %q, want %q via .git redirect", got, "wt")
	}
}
