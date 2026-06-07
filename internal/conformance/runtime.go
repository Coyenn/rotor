package conformance

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type RuntimeTools struct {
	Rojo string
	Lune string
}

func detectRuntimeTools() RuntimeTools {
	return RuntimeTools{
		Rojo: lookPath("rojo"),
		Lune: lookPath("lune"),
	}
}

func lookPath(name string) string {
	path, err := exec.LookPath(name)
	if err != nil {
		return ""
	}
	return path
}

func runBehavioralSuite(root string) error {
	projDir := filepath.Join(root, "testdata", "conformance", "project")
	if err := ensureConformanceProjectDeps(projDir); err != nil {
		return err
	}

	buildCmd := exec.Command("go", "run", "./cmd/rotor", "build", "./testdata/conformance/project", "--type", "model")
	buildCmd.Dir = root
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("rotor build conformance project: %w", err)
	}

	placePath := filepath.Join(projDir, "conformance-tests.rbxlx")
	rojoCmd := exec.Command("rojo", "build", "--output", placePath, filepath.Join(projDir, "default.project.json"))
	rojoCmd.Dir = projDir
	rojoCmd.Stdout = os.Stdout
	rojoCmd.Stderr = os.Stderr
	if err := rojoCmd.Run(); err != nil {
		return fmt.Errorf("rojo build conformance project: %w", err)
	}

	luneCmd := exec.Command("lune", "run", filepath.Join(root, "reference", "roblox-ts", "tests", "runTestsWithLune.lua"), placePath)
	luneCmd.Dir = root
	luneCmd.Stdout = os.Stdout
	luneCmd.Stderr = os.Stderr
	if err := luneCmd.Run(); err != nil {
		return fmt.Errorf("lune behavioral suite: %w", err)
	}

	return nil
}
