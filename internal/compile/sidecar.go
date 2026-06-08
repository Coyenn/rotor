package compile

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"rotor/internal/logservice"
	"rotor/tsgo/ast"
	"rotor/tsgo/bundled"
	"rotor/tsgo/compiler"
	"rotor/tsgo/tsoptions"
	"rotor/tsgo/vfs"
	"rotor/tsgo/vfs/osvfs"
	"rotor/tsgo/vfs/wrapvfs"
)

type sidecarRequest struct {
	Protocol         int                  `json:"protocol"`
	TsConfigPath     string               `json:"tsConfigPath"`
	ProjectDir       string               `json:"projectDir"`
	CompileFileNames []string             `json:"compileFileNames"`
	ChangedFiles     []sidecarChangedFile `json:"changedFiles"`
}

type sidecarChangedFile struct {
	FileName string `json:"fileName"`
	Text     string `json:"text"`
}

type sidecarResponse struct {
	Diagnostics []sidecarDiagnostic `json:"diagnostics"`
	Transformed []sidecarOutputFile `json:"transformed"`
}

type sidecarDiagnostic struct {
	Category string `json:"category"`
	Code     string `json:"code"`
	File     string `json:"file"`
	Start    int    `json:"start"`
	Length   int    `json:"length"`
	Message  string `json:"message"`
}

type sidecarOutputFile struct {
	FileName string `json:"fileName"`
	Text     string `json:"text"`
}

var (
	sidecarMainPath = repoFile("tools", "sidecar", "main.js")
	sidecarNodePath = repoFile("testdata", "diff", "project", "node_modules")
)

func prepareProjectProgramForCompile(dir string, program *compiler.Program, sourceFiles []*ast.SourceFile) (*compiler.Program, []*ast.SourceFile, []string, error) {
	if len(sourceFiles) == 0 {
		return program, sourceFiles, nil, nil
	}
	if !projectUsesTransformerPlugins(program.CommandLine()) {
		return program, sourceFiles, nil, nil
	}

	transformedProgram, diags, err := applyTransformerSidecar(dir, program, sourceFiles)
	if err != nil {
		return nil, nil, diags, err
	}
	if transformedProgram == program {
		return program, sourceFiles, nil, nil
	}

	remapped, err := remapProgramSourceFiles(transformedProgram, sourceFiles)
	if err != nil {
		return nil, nil, nil, err
	}
	return transformedProgram, remapped, nil, nil
}

func applyTransformerSidecar(dir string, program *compiler.Program, sourceFiles []*ast.SourceFile) (*compiler.Program, []string, error) {
	configPath := program.Options().ConfigFilePath
	if configPath == "" {
		configPath = filepath.ToSlash(filepath.Join(filepath.FromSlash(dir), "tsconfig.json"))
	}

	response, err := runTransformerSidecar(dir, configPath, sourceFiles)
	if err != nil {
		return nil, []string{err.Error()}, err
	}

	var errorDiags []string
	for _, diag := range response.Diagnostics {
		text := formatSidecarDiagnostic(diag)
		if strings.EqualFold(diag.Category, "warning") {
			logservice.Warn(text)
			continue
		}
		errorDiags = append(errorDiags, text)
	}
	if len(errorDiags) > 0 {
		return nil, errorDiags, errors.New("compile: transformer sidecar diagnostics")
	}
	if len(response.Transformed) == 0 {
		return program, nil, nil
	}

	overlays := make(map[string]string, len(response.Transformed))
	caseSensitive := osvfs.FS().UseCaseSensitiveFileNames()
	for _, file := range response.Transformed {
		overlays[normalizeOverlayPath(file.FileName, caseSensitive)] = file.Text
	}
	return newProjectProgramWithOverlay(dir, configPath, overlays)
}

func runTransformerSidecar(dir, configPath string, sourceFiles []*ast.SourceFile) (*sidecarResponse, error) {
	nodeCommand := os.Getenv("ROTOR_NODE_PATH")
	if nodeCommand != "" {
		if _, err := os.Stat(nodeCommand); err != nil {
			return nil, errors.New("node executable not found; rotor transformer plugins require Node.js on PATH")
		}
	} else {
		nodeCommand = "node"
	}
	nodePath, err := exec.LookPath(nodeCommand)
	if err != nil {
		return nil, errors.New("node executable not found; rotor transformer plugins require Node.js on PATH")
	}

	request := sidecarRequest{
		Protocol:         1,
		TsConfigPath:     filepath.FromSlash(configPath),
		ProjectDir:       filepath.FromSlash(dir),
		CompileFileNames: make([]string, 0, len(sourceFiles)),
		ChangedFiles:     []sidecarChangedFile{},
	}
	for _, sourceFile := range sourceFiles {
		request.CompileFileNames = append(request.CompileFileNames, filepath.FromSlash(sourceFile.FileName()))
	}

	payload, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(nodePath, filepath.FromSlash(sidecarMainPath))
	cmd.Dir = filepath.FromSlash(dir)
	cmd.Env = sidecarEnv(dir)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	if _, err := stdin.Write(append(payload, '\n')); err != nil {
		_ = stdin.Close()
		_ = cmd.Wait()
		return nil, err
	}
	if err := stdin.Close(); err != nil {
		_ = cmd.Wait()
		return nil, err
	}

	line, err := bufio.NewReader(stdout).ReadBytes('\n')
	if err != nil {
		_ = cmd.Wait()
		if stderr.Len() > 0 {
			return nil, fmt.Errorf("transformer sidecar failed: %s", strings.TrimSpace(stderr.String()))
		}
		return nil, err
	}

	if err := cmd.Wait(); err != nil {
		if stderr.Len() > 0 {
			return nil, fmt.Errorf("transformer sidecar failed: %s", strings.TrimSpace(stderr.String()))
		}
		return nil, err
	}

	var response sidecarResponse
	if err := json.Unmarshal(bytes.TrimSpace(line), &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func sidecarEnv(projectDir string) []string {
	nodePaths := []string{
		filepath.Join(filepath.FromSlash(projectDir), "node_modules"),
		filepath.FromSlash(filepath.Join(filepath.FromSlash(filepath.Dir(sidecarMainPath)), "node_modules")),
		filepath.FromSlash(sidecarNodePath),
	}

	var filtered []string
	seen := map[string]struct{}{}
	for _, path := range nodePaths {
		if path == "" {
			continue
		}
		path = filepath.Clean(path)
		if _, err := os.Stat(path); err != nil {
			continue
		}
		key := path
		if runtime.GOOS == "windows" {
			key = strings.ToLower(key)
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		filtered = append(filtered, path)
	}

	env := os.Environ()
	if len(filtered) == 0 {
		return env
	}

	nodePathValue := strings.Join(filtered, string(os.PathListSeparator))
	for i, entry := range env {
		if strings.HasPrefix(entry, "NODE_PATH=") {
			if existing := strings.TrimPrefix(entry, "NODE_PATH="); existing != "" {
				nodePathValue += string(os.PathListSeparator) + existing
			}
			env[i] = "NODE_PATH=" + nodePathValue
			return env
		}
	}
	return append(env, "NODE_PATH="+nodePathValue)
}

func newProjectProgramWithOverlay(projectDir, tsConfigPath string, overlays map[string]string) (*compiler.Program, []string, error) {
	dir := filepath.ToSlash(projectDir)
	if abs, err := filepath.Abs(projectDir); err == nil {
		dir = filepath.ToSlash(abs)
	}
	configPath := filepath.ToSlash(tsConfigPath)
	if abs, err := filepath.Abs(tsConfigPath); err == nil {
		configPath = filepath.ToSlash(abs)
	}

	baseFS := SanitizeFSWithConfigPath(bundled.WrapFS(osvfs.FS()), configPath)
	caseSensitive := baseFS.UseCaseSensitiveFileNames()
	fs := wrapvfs.Wrap(baseFS, wrapvfs.Replacements{
		FileExists: func(path string) bool {
			if _, ok := overlays[normalizeOverlayPath(path, caseSensitive)]; ok {
				return true
			}
			return baseFS.FileExists(path)
		},
		ReadFile: func(path string) (string, bool) {
			if text, ok := overlays[normalizeOverlayPath(path, caseSensitive)]; ok {
				return text, true
			}
			return baseFS.ReadFile(path)
		},
	})
	return newProjectProgramFromFS(dir, configPath, fs)
}

func newProjectProgramFromFS(dir, configPath string, fs vfs.FS) (*compiler.Program, []string, error) {
	host := compiler.NewCompilerHost(dir, fs, bundled.LibPath(), nil, nil)
	parsed, configDiags := tsoptions.GetParsedCommandLineOfConfigFile(configPath, nil, nil, host, nil)
	if len(configDiags) > 0 {
		return nil, diagnosticStrings(configDiags), errors.New("compile: tsconfig.json has errors")
	}

	raw := readRawEnforcedOptions(filepath.FromSlash(configPath))
	if msg := validateCompilerOptions(parsed.CompilerOptions(), dir, raw); msg != "" {
		return nil, []string{msg}, errors.New("compile: invalid tsconfig.json configuration")
	}

	return compiler.NewProgram(compiler.ProgramOptions{
		Host:   host,
		Config: parsed,
	}), nil, nil
}

func remapProgramSourceFiles(program *compiler.Program, sourceFiles []*ast.SourceFile) ([]*ast.SourceFile, error) {
	byPath := make(map[string]*ast.SourceFile)
	for _, sourceFile := range projectSourceFiles(program) {
		byPath[normalizeSourceFilePath(sourceFile.FileName())] = sourceFile
	}

	remapped := make([]*ast.SourceFile, 0, len(sourceFiles))
	for _, sourceFile := range sourceFiles {
		path := normalizeSourceFilePath(sourceFile.FileName())
		mapped := byPath[path]
		if mapped == nil {
			return nil, fmt.Errorf("compile: transformed source file missing from overlay program: %s", path)
		}
		remapped = append(remapped, mapped)
	}
	return remapped, nil
}

func projectUsesTransformerPlugins(parsed *tsoptions.ParsedCommandLine) bool {
	if parsed == nil {
		return false
	}
	configFiles := append([]string{parsed.ConfigName()}, parsed.ExtendedSourceFiles()...)
	seen := map[string]struct{}{}
	for _, configPath := range configFiles {
		if configPath == "" {
			continue
		}
		path := normalizeSourceFilePath(configPath)
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		if configFileUsesTransformerPlugins(path) {
			return true
		}
	}
	return false
}

func configFileUsesTransformerPlugins(configPath string) bool {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return false
	}

	var root map[string]any
	if json.Unmarshal([]byte(stripJSONC(string(data))), &root) != nil {
		return false
	}
	compilerOptions, ok := root["compilerOptions"].(map[string]any)
	if !ok {
		return false
	}
	plugins, ok := compilerOptions["plugins"].([]any)
	if !ok {
		return false
	}
	for _, plugin := range plugins {
		if pluginConfig, ok := plugin.(map[string]any); ok {
			if transform, ok := pluginConfig["transform"].(string); ok && transform != "" {
				return true
			}
		}
	}
	return false
}

func formatSidecarDiagnostic(diag sidecarDiagnostic) string {
	if diag.File != "" {
		return filepath.FromSlash(diag.File) + ": " + diag.Message
	}
	return diag.Message
}

func normalizeOverlayPath(path string, caseSensitive bool) string {
	path = normalizeSourceFilePath(path)
	if !caseSensitive {
		path = strings.ToLower(path)
	}
	return path
}

func repoFile(parts ...string) string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return filepath.Join(parts...)
	}
	segments := append([]string{filepath.Dir(file), "..", ".."}, parts...)
	return filepath.Clean(filepath.Join(segments...))
}
