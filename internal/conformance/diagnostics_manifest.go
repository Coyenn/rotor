package conformance

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"

	"rotor/internal/compile"
)

type DiagnosticFixture struct {
	Name       string
	Path       string
	ExpectedID string
}

var skippedDiagnosticFixtures = map[string]string{
	"noIsolatedImport.ts": "requires an isolated Rojo container topology that the model conformance project intentionally does not model",
	"noRojoData.ts":       "requires a missing-Rojo-data project layout that the model conformance project intentionally does not model",
}

var skippedDiagnosticIDs = map[string]string{}

var installConformanceDepsOnce struct {
	once sync.Once
	err  error
}

func loadDiagnosticFixtures(dir string) ([]DiagnosticFixture, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var fixtures []DiagnosticFixture
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		switch filepath.Ext(name) {
		case ".ts", ".tsx":
		default:
			continue
		}
		fixtures = append(fixtures, DiagnosticFixture{
			Name:       name,
			Path:       filepath.Join(dir, name),
			ExpectedID: diagnosticBaseName(name),
		})
	}
	slices.SortFunc(fixtures, func(a, b DiagnosticFixture) int {
		return strings.Compare(a.Name, b.Name)
	})
	return fixtures, nil
}

func compileDiagnosticFixture(root, fixturePath string) ([]string, error) {
	projectDir := filepath.Join(root, "testdata", "conformance", "project")
	if err := ensureConformanceProjectDeps(projectDir); err != nil {
		return nil, err
	}

	overlayDir := filepath.Join(projectDir, "src", "__diagnostics")
	if err := os.MkdirAll(overlayDir, 0o755); err != nil {
		return nil, err
	}

	name := filepath.Base(fixturePath)
	overlayPath := filepath.Join(overlayDir, name)
	data, err := os.ReadFile(fixturePath)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(overlayPath, data, 0o644); err != nil {
		return nil, err
	}
	defer func() {
		_ = os.Remove(overlayPath)
		_ = os.Remove(overlayDir)
	}()

	_, diags, err := compile.CompileFileDetailedWithOptions(projectDir, filepath.ToSlash(filepath.Join("src", "__diagnostics", name)), compile.ProjectOptions{
		AllowCommentDirectives: true,
	})
	if err != nil && len(diags) == 0 {
		return nil, err
	}

	ids := make([]string, 0, len(diags))
	for _, diag := range diags {
		ids = append(ids, diag.Code)
	}
	return ids, nil
}

func diagnosticBaseName(name string) string {
	base := strings.TrimSuffix(strings.TrimSuffix(name, ".tsx"), ".ts")
	if dot := strings.LastIndexByte(base, '.'); dot >= 0 {
		if _, err := strconv.Atoi(base[dot+1:]); err == nil {
			base = base[:dot]
		}
	}
	return base
}

func ensureConformanceProjectDeps(projectDir string) error {
	installConformanceDepsOnce.once.Do(func() {
		if _, err := os.Stat(filepath.Join(projectDir, "node_modules")); err == nil {
			return
		}
		cmd := exec.Command("npm", "install", "--no-audit", "--no-fund")
		cmd.Dir = projectDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		installConformanceDepsOnce.err = cmd.Run()
		if installConformanceDepsOnce.err != nil {
			installConformanceDepsOnce.err = fmt.Errorf("install conformance project dependencies: %w", installConformanceDepsOnce.err)
		}
	})
	return installConformanceDepsOnce.err
}

func skipReasonForDiagnosticFixture(tc DiagnosticFixture) string {
	if reason, ok := skippedDiagnosticFixtures[tc.Name]; ok {
		return reason
	}
	return skippedDiagnosticIDs[tc.ExpectedID]
}
