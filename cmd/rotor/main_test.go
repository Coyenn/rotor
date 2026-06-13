package main

import (
	"strings"
	"testing"
)

func TestVersionCommandPrintsInjectedVersion(t *testing.T) {
	old := version
	version = "9.9.9-test"
	t.Cleanup(func() { version = old })

	output, code := captureStdout(t, func() int {
		return run([]string{"--version"})
	})
	if code != 0 {
		t.Fatalf("run exit = %d, want 0", code)
	}

	if strings.TrimSpace(output) != "9.9.9-test" {
		t.Fatalf("version output = %q, want %q", output, "9.9.9-test")
	}
}
