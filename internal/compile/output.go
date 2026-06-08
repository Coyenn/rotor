package compile

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"rotor/internal/includefiles"
	"rotor/internal/logservice"
	"rotor/internal/rojo"
	"rotor/tsgo/compiler"
)

// BuildResult is the disk-writing sibling of CompileProject's pure text map.
// Outputs contains the compiled Luau sources keyed by project-relative output
// path; EmittedFiles contains the compiled output paths actually written to
// disk this pass (mirroring compileFiles.ts' emittedFiles and excluding copied
// passthrough files).
type BuildResult struct {
	Outputs      map[string]string
	EmittedFiles []string
	OutputDir    string
}

// BuildProjectWithOptions runs the Phase 4 output pipeline for `rotor build`:
// cleanup -> copyInclude -> copy non-compiled files -> compile -> write
// compiled outputs. CompileProject remains the pure library API; this is the
// writing entry point for the CLI and future watch/incremental layers.
func BuildProjectWithOptions(projectDir string, opts ProjectOptions) (*BuildResult, []string, error) {
	dir, program, diags, err := newProjectProgram(projectDir, opts.TsConfigPath)
	if err != nil {
		return nil, diags, err
	}

	pathTranslator := createPathTranslator(program, !opts.LuaExtension)
	cleanupOutputs(pathTranslator)

	if err := maybeCopyInclude(dir, opts); err != nil {
		return nil, nil, err
	}
	if err := copyNonCompiledFiles(pathTranslator, getRootDirs(program), opts.WriteOnlyChanged); err != nil {
		return nil, nil, err
	}

	sourceFiles := projectSourceFiles(program)
	selectedFiles := sourceFiles
	var previousManifest *incrementalManifest
	var currentManifest *incrementalManifest
	if program.Options().Incremental.IsTrue() && pathTranslator.BuildInfoOutputPath != "" {
		previousManifest, err = readIncrementalManifest(pathTranslator.BuildInfoOutputPath)
		if err != nil {
			return nil, nil, err
		}
		currentManifest, err = buildIncrementalManifest(program, sourceFiles, incrementalSalt(program, opts, pathTranslator.BuildInfoOutputPath))
		if err != nil {
			return nil, nil, err
		}
		selectedFiles = selectIncrementalSourceFiles(sourceFiles, currentManifest, previousManifest)
	}

	pctx, diags, err := newProjectContext(dir, program, opts)
	if err != nil {
		return nil, diags, err
	}
	outputs, diags, err := compileProjectSourceFiles(dir, program, pctx, selectedFiles, opts)
	if err != nil {
		return nil, diags, err
	}

	emittedFiles := make([]string, 0, len(outputs))
	relOuts := make([]string, 0, len(outputs))
	for relOut := range outputs {
		relOuts = append(relOuts, relOut)
	}
	sort.Strings(relOuts)

	for _, relOut := range relOuts {
		absOut := filepath.Join(filepath.FromSlash(dir), filepath.FromSlash(relOut))
		wrote, err := writeOutputFile(absOut, outputs[relOut], opts.WriteOnlyChanged)
		if err != nil {
			return nil, nil, err
		}
		if wrote {
			emittedFiles = append(emittedFiles, absOut)
		}
	}

	selectedPaths := make(map[string]struct{}, len(selectedFiles))
	for _, sourceFile := range selectedFiles {
		selectedPaths[normalizeSourceFilePath(sourceFile.FileName())] = struct{}{}
	}

	declFiles, err := emitDeclarations(program, selectedPaths, opts.WriteOnlyChanged)
	if err != nil {
		return nil, nil, err
	}
	emittedFiles = append(emittedFiles, declFiles...)

	if currentManifest != nil && !sameIncrementalManifest(previousManifest, currentManifest) {
		if err := writeIncrementalManifest(pathTranslator.BuildInfoOutputPath, currentManifest); err != nil {
			return nil, nil, err
		}
	}

	return &BuildResult{
		Outputs:      outputs,
		EmittedFiles: emittedFiles,
		OutputDir:    pathTranslator.OutDir,
	}, nil, nil
}

func emitDeclarations(program *compiler.Program, selectedPaths map[string]struct{}, writeOnlyChanged bool) ([]string, error) {
	if !program.Options().Declaration.IsTrue() {
		return nil, nil
	}

	ctx := context.Background()
	var emittedFiles []string
	for _, sourceFile := range program.SourceFiles() {
		if sourceFile.IsDeclarationFile || !isCompilableFile(sourceFile.FileName()) {
			continue
		}
		if selectedPaths != nil {
			if _, ok := selectedPaths[normalizeSourceFilePath(sourceFile.FileName())]; !ok {
				continue
			}
		}
		result := program.Emit(ctx, compiler.EmitOptions{
			TargetSourceFile: sourceFile,
			EmitOnly:         compiler.EmitOnlyDts,
			WriteFile: func(fileName string, text string, data *compiler.WriteFileData) error {
				text = rewriteDeclarationTypeReferences(text)
				wrote, err := writeOutputFile(filepath.FromSlash(fileName), text, writeOnlyChanged)
				if !wrote && data != nil {
					data.SkippedDtsWrite = true
				}
				return err
			},
		})
		if result == nil {
			continue
		}
		if len(result.Diagnostics) > 0 {
			return nil, errors.New("compile: declaration emit diagnostics")
		}
		emittedFiles = append(emittedFiles, result.EmittedFiles...)
	}
	return emittedFiles, nil
}

func rewriteDeclarationTypeReferences(text string) string {
	return strings.ReplaceAll(text, `types="types"`, `types="@rbxts/types"`)
}

func writeOutputFile(path string, text string, writeOnlyChanged bool) (bool, error) {
	if writeOnlyChanged {
		if existing, err := os.ReadFile(path); err == nil && string(existing) == text {
			return false, nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		return false, err
	}
	return true, nil
}

func maybeCopyInclude(dir string, opts ProjectOptions) error {
	if !opts.EmitIncludeFiles || opts.Type == "package" {
		return nil
	}
	_, isPackage, err := projectIsPackage(dir)
	if err != nil {
		return err
	}
	if opts.Type == "" && isPackage {
		return nil
	}

	includePath, err := resolveIncludePath(dir, opts.IncludePath)
	if err != nil {
		return err
	}
	var copyErr error
	logservice.BenchmarkIfVerbose("copy include files", func() {
		copyErr = includefiles.Copy(includePath)
	})
	return copyErr
}

func cleanupOutputs(pathTranslator *rojo.PathTranslator) {
	cleanupDirRecursively(pathTranslator, pathTranslator.OutDir)
}

func cleanupDirRecursively(pathTranslator *rojo.PathTranslator, dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		itemPath := filepath.Join(dir, entry.Name())
		if entry.IsDir() {
			if entry.Name() == ".git" {
				continue
			}
			cleanupDirRecursively(pathTranslator, itemPath)
		}
		tryRemoveOutput(pathTranslator, itemPath)
	}
}

func tryRemoveOutput(pathTranslator *rojo.PathTranslator, outPath string) {
	if !isOutputFileOrphaned(pathTranslator, outPath) {
		return
	}
	if err := os.RemoveAll(outPath); err == nil {
		logservice.WriteLineIfVerbose("remove " + outPath)
	}
}

func isOutputFileOrphaned(pathTranslator *rojo.PathTranslator, outPath string) bool {
	if strings.HasSuffix(outPath, ".d.ts") && !pathTranslator.Declaration {
		return true
	}
	for _, inputPath := range pathTranslator.GetInputPaths(outPath) {
		if _, err := os.Stat(inputPath); err == nil {
			return false
		}
	}
	if pathTranslator.BuildInfoOutputPath == outPath {
		return false
	}
	return true
}

func copyNonCompiledFiles(pathTranslator *rojo.PathTranslator, rootDirs []string, writeOnlyChanged bool) error {
	for _, rootDir := range rootDirs {
		rootDir = filepath.FromSlash(rootDir)
		if _, err := os.Stat(rootDir); err != nil {
			continue
		}
		if err := copyItem(pathTranslator, rootDir, writeOnlyChanged); err != nil {
			return err
		}
	}
	return nil
}

func copyItem(pathTranslator *rojo.PathTranslator, itemPath string, writeOnlyChanged bool) error {
	info, err := os.Stat(itemPath)
	if err != nil {
		return err
	}
	if info.IsDir() {
		outDir := pathTranslator.GetOutputPath(itemPath)
		if err := os.MkdirAll(outDir, 0o755); err != nil {
			return err
		}
		entries, err := os.ReadDir(itemPath)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if err := copyItem(pathTranslator, filepath.Join(itemPath, entry.Name()), writeOnlyChanged); err != nil {
				return err
			}
		}
		return nil
	}

	if strings.HasSuffix(itemPath, ".d.ts") {
		if !pathTranslator.Declaration {
			return nil
		}
	} else if isCompilableFile(itemPath) {
		return nil
	}

	dest := pathTranslator.GetOutputPath(itemPath)
	if writeOnlyChanged {
		if existing, err := os.ReadFile(dest); err == nil {
			incoming, err := os.ReadFile(itemPath)
			if err != nil {
				return err
			}
			if string(existing) == string(incoming) {
				return nil
			}
		}
	}

	data, err := os.ReadFile(itemPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dest, data, 0o644)
}

func isCompilableFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	if strings.HasSuffix(path, ".d.ts") {
		return false
	}
	return strings.HasSuffix(path, ".ts") || strings.HasSuffix(path, ".tsx")
}
