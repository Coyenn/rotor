package main

import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestVersionCommandPrintsInjectedVersion(t *testing.T) {
	old := version
	version = "9.9.9-test"
	t.Cleanup(func() { version = old })

	output := captureStdout(t, func() {
		if code := run([]string{"--version"}); code != 0 {
			t.Fatalf("run exit = %d, want 0", code)
		}
	})

	if strings.TrimSpace(output) != "9.9.9-test" {
		t.Fatalf("version output = %q, want %q", output, "9.9.9-test")
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = old })

	fn()

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
