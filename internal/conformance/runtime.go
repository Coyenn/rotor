package conformance

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

type RuntimeTools struct {
	Rojo string
	Lune string
}

var DisabledBehavioralFixtures = map[string]string{
	"tests/roact_spread.spec.luau": "runtime failure under Lune: Roact strict table lacks the expected jsx member",
}

func detectRuntimeTools() RuntimeTools {
	return RuntimeTools{
		Rojo: detectRuntimeTool("ROTOR_ROJO_PATH", "rojo"),
		Lune: detectRuntimeTool("ROTOR_LUNE_PATH", "lune"),
	}
}

func detectRuntimeTool(envKey, binary string) string {
	if override := strings.TrimSpace(os.Getenv(envKey)); override != "" {
		if info, err := os.Stat(override); err == nil && !info.IsDir() {
			return override
		}
		return ""
	}
	return lookPath(binary)
}

func lookPath(name string) string {
	path, err := exec.LookPath(name)
	if err != nil {
		return ""
	}
	return path
}

func runtimeSuiteSkipReason(tools RuntimeTools) string {
	var missing []string
	var hints []string
	if tools.Rojo == "" {
		missing = append(missing, "rojo")
		hints = append(hints, "set ROTOR_ROJO_PATH to a rojo executable or add rojo to PATH")
	}
	if tools.Lune == "" {
		missing = append(missing, "lune")
		hints = append(hints, "set ROTOR_LUNE_PATH to a lune executable or add lune to PATH")
	}
	if len(missing) == 0 {
		return ""
	}
	return fmt.Sprintf("runtime suite requires %s; %s", strings.Join(missing, " and "), strings.Join(hints, "; "))
}

func runtimeSuiteSourceRels(projectDir string) ([]string, error) {
	rels := []string{
		"main.server.ts",
		"services.d.ts",
	}
	for _, goldenRel := range EnabledFixtures {
		if !strings.HasPrefix(goldenRel, "tests/") {
			continue
		}
		if _, disabled := DisabledBehavioralFixtures[goldenRel]; disabled {
			continue
		}
		sourceRel, err := sourceRelFromGolden(projectDir, goldenRel)
		if err != nil {
			return nil, err
		}
		rels = append(rels, sourceRel)
	}
	slices.Sort(rels)
	return rels, nil
}

func stageRuntimeSuiteProject(baseProjectDir string) (string, error) {
	tmpProj, err := os.MkdirTemp(baseProjectDir, ".phase5-runtime-")
	if err != nil {
		return "", err
	}
	if err := copyFile(filepath.Join(baseProjectDir, "package.json"), filepath.Join(tmpProj, "package.json")); err != nil {
		return "", err
	}
	if err := copyFile(filepath.Join(baseProjectDir, "tsconfig.json"), filepath.Join(tmpProj, "tsconfig.json")); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(tmpProj, "default.project.json"), []byte(runtimeRojoConfig()), 0o644); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Join(tmpProj, "src"), 0o755); err != nil {
		return "", err
	}
	if err := ensureFixtureTypeRoots(baseProjectDir, tmpProj); err != nil {
		return "", err
	}
	if err := copyTree(filepath.Join(baseProjectDir, "src", "helpers"), filepath.Join(tmpProj, "src", "helpers")); err != nil {
		return "", err
	}

	sourceRels, err := runtimeSuiteSourceRels(baseProjectDir)
	if err != nil {
		return "", err
	}
	for _, rel := range sourceRels {
		if err := copyFile(filepath.Join(baseProjectDir, "src", filepath.FromSlash(rel)), filepath.Join(tmpProj, "src", filepath.FromSlash(rel))); err != nil {
			return "", err
		}
	}
	return tmpProj, nil
}

func runtimeRojoConfig() string {
	return `{
	"name": "conformance-runtime",
	"tree": {
		"$className": "DataModel",
		"ReplicatedStorage": {
			"$className": "ReplicatedStorage",
			"include": {
				"$path": "include",
				"node_modules": {
					"$className": "Folder",
					"@rbxts": {
						"$path": "node_modules/@rbxts"
					}
				}
			}
		},
		"ServerScriptService": {
			"$className": "ServerScriptService",
			"main": {
				"$path": "out/main.server.luau"
			},
			"tests": {
				"$path": "out/tests"
			},
			"helpers": {
				"$className": "Folder",
				"util": {
					"$path": "out/helpers/util"
				}
			}
		},
		"StarterGui": {
			"$className": "StarterGui",
			"isolated": {
				"$path": "out/helpers/rojo/isolated.luau"
			}
		}
	}
}`
}

func runBehavioralSuite(root string, tools RuntimeTools) error {
	baseProjectDir := filepath.Join(root, "testdata", "conformance", "project")
	if err := ensureConformanceProjectDeps(baseProjectDir); err != nil {
		return err
	}

	projDir, err := stageRuntimeSuiteProject(baseProjectDir)
	if err != nil {
		return err
	}
	defer os.RemoveAll(projDir)

	buildCmd := exec.Command("go", "run", "./cmd/rotor", "build", projDir, "--type", "game", "--allowCommentDirectives")
	buildCmd.Dir = root
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("rotor build conformance project: %w", err)
	}

	placePath := filepath.Join(projDir, "conformance-tests.rbxlx")
	rojoCmd := exec.Command(tools.Rojo, "build", "--output", placePath, filepath.Join(projDir, "default.project.json"))
	rojoCmd.Dir = projDir
	rojoCmd.Stdout = os.Stdout
	rojoCmd.Stderr = os.Stderr
	if err := rojoCmd.Run(); err != nil {
		return fmt.Errorf("rojo build conformance project: %w", err)
	}

	luneCmd := exec.Command(tools.Lune, "run", filepath.Join(root, "reference", "roblox-ts", "tests", "runTestsWithLune.lua"), placePath)
	luneCmd.Dir = root
	luneCmd.Stdout = os.Stdout
	luneCmd.Stderr = os.Stderr
	if err := luneCmd.Run(); err != nil {
		return fmt.Errorf("lune behavioral suite: %w", err)
	}

	return nil
}
