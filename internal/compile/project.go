package compile

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"rotor/internal/rojo"
	"rotor/internal/transformer"
	"rotor/tsgo/bundled"
	"rotor/tsgo/compiler"
	"rotor/tsgo/tsoptions"
	"rotor/tsgo/vfs/osvfs"
)

// packageNameRegex ports createProjectData.ts PACKAGE_REGEX: a project whose
// package.json name has an npm scope compiles as ProjectType.Package.
var packageNameRegex = regexp.MustCompile(`^@[a-z0-9-]*/`)

// filenameWarnings ports Shared/constants.ts FILENAME_WARNINGS: `init.*` file
// names collide with Rojo's directory-script convention; checkFileName
// suggests the `index.*` spelling.
var filenameWarnings = func() map[string]string {
	m := make(map[string]string)
	for _, scriptType := range []string{".server", ".client", ""} {
		for _, fileType := range []string{".ts", ".tsx", ".d.ts"} {
			m["init"+scriptType+fileType] = "index" + scriptType + fileType
		}
	}
	return m
}()

// projectContext is the Go analog of upstream ProjectData plus the
// compileFiles.ts locals computed once per compilation pass (L49-98):
// RojoResolver, PathTranslator, inferred ProjectType, and the validated
// runtimeLibRbxPath, packaged as the transformer's RojoContext.
type projectContext struct {
	dir         string // abs slash project dir (upstream projectPath)
	projectType transformer.ProjectType
	rojoContext *transformer.RojoContext
}

// newProjectProgram builds the tsgo Program for projectDir over the sanitized
// config — the shared front half of CompileFile and CompileProject.
func newProjectProgram(projectDir string) (string, *compiler.Program, []string, error) {
	dir, err := filepath.Abs(projectDir)
	if err != nil {
		return "", nil, nil, err
	}
	dir = filepath.ToSlash(dir)

	fs := SanitizeFS(bundled.WrapFS(osvfs.FS()))
	host := compiler.NewCompilerHost(dir, fs, bundled.LibPath(), nil, nil)

	configPath := dir + "/tsconfig.json"
	parsed, configDiags := tsoptions.GetParsedCommandLineOfConfigFile(configPath, nil, nil, host, nil)
	if len(configDiags) > 0 {
		return "", nil, diagnosticStrings(configDiags), errors.New("compile: tsconfig.json has errors")
	}

	program := compiler.NewProgram(compiler.ProgramOptions{
		Host:   host,
		Config: parsed,
	})
	return dir, program, nil, nil
}

// newProjectContext ports the project-level setup of compileFiles.ts L56-100
// (with createProjectData.ts feeding it): RojoResolver construction,
// checkRojoConfig/checkFileName, ProjectType inference, and runtimeLibRbxPath
// discovery + validation. The four plain-text emit failures (compileFiles.ts
// L83-96) are hard errors, returned as diagnostics alongside the error.
//
// ProjectType inference, as upstream (rotor has no --type CLI flag yet, so
// the `data.projectOptions.type ??` override never applies):
//   - package.json name has an npm scope (PACKAGE_REGEX)  -> Package
//   - the Rojo tree declares $className DataModel (isGame) -> Game
//   - otherwise                                            -> Model
func newProjectContext(dir string, program *compiler.Program) (*projectContext, []string, error) {
	options := program.Options()
	outDir := options.OutDir

	// createProjectData.ts L15-31: package.json discovery walks up from the
	// project path (ts.findPackageJson); a missing package.json is an error,
	// an unreadable one just means "not a package".
	pkgJSONPath := findPackageJSON(dir)
	if pkgJSONPath == "" {
		return nil, nil, errors.New("compile: Unable to find package.json")
	}
	isPackage := false
	if data, err := os.ReadFile(pkgJSONPath); err == nil {
		var pkg struct {
			Name string `json:"name"`
		}
		if json.Unmarshal(data, &pkg) == nil {
			isPackage = packageNameRegex.MatchString(pkg.Name)
		}
	}

	// includePath: DEFAULT_PROJECT_OPTIONS.includePath is "" and rotor has no
	// --includePath flag, so createProjectData.ts L29's `||` fallback always
	// resolves <projectPath>/include.
	includePath := filepath.Join(filepath.FromSlash(dir), "include")

	// Upstream logs FindRojoConfigFilePath/resolver warnings via
	// LogService.warn and proceeds; rotor has no warning channel here yet, so
	// they are intentionally dropped (they never fail a compile upstream).
	rojoConfigPath, _ := rojo.FindRojoConfigFilePath(filepath.FromSlash(dir))

	// compileFiles.ts L61-63.
	var rojoResolver *rojo.RojoResolver
	if rojoConfigPath != "" {
		rojoResolver = rojo.FromPath(rojoConfigPath)
	} else {
		rojoResolver = rojo.Synthetic(filepath.FromSlash(outDir))
	}

	pathTranslator := createPathTranslator(program)

	// checkRojoConfig + checkFileName queue project-level diagnostics
	// (compileFiles.ts L69-75); upstream flushes them only after the emit
	// failures below get their early returns, so the queue is checked last.
	var checkDiags []string
	checkDiags = append(checkDiags, checkRojoConfig(rojoConfigPath, rojoResolver, getRootDirs(program), pathTranslator)...)
	nodeModulesPath := filepath.Join(filepath.Dir(pkgJSONPath), "node_modules")
	for _, sourceFile := range program.SourceFiles() {
		fileName := filepath.FromSlash(sourceFile.FileName())
		if !strings.HasPrefix(fileName, nodeModulesPath) {
			if msg := checkFileName(fileName); msg != "" {
				checkDiags = append(checkDiags, msg)
			}
		}
	}

	// compileFiles.ts L80: inferProjectType (no projectOptions.type override).
	var projectType transformer.ProjectType
	switch {
	case isPackage:
		projectType = transformer.ProjectTypePackage
	case rojoResolver.IsGame:
		projectType = transformer.ProjectTypeGame
	default:
		projectType = transformer.ProjectTypeModel
	}

	// The four plain-text emit failures (compileFiles.ts L82-98) — hard
	// errors per digest §7/§8.
	if projectType != transformer.ProjectTypePackage && rojoConfigPath == "" {
		return nil, []string{"Non-package projects must have a Rojo project file!"}, errors.New("compile: emit failure")
	}
	var runtimeLibRbxPath rojo.RbxPath
	if projectType != transformer.ProjectTypePackage {
		var ok bool
		runtimeLibRbxPath, ok = rojoResolver.GetRbxPathFromFilePath(filepath.Join(includePath, "RuntimeLib.lua"))
		if !ok {
			return nil, []string{"Rojo project contained no data for include folder!"}, errors.New("compile: emit failure")
		} else if rojoResolver.GetNetworkType(runtimeLibRbxPath) != rojo.NetworkTypeUnknown {
			return nil, []string{"Runtime library cannot be in a server-only or client-only container!"}, errors.New("compile: emit failure")
		} else if rojoResolver.IsIsolated(runtimeLibRbxPath) {
			return nil, []string{"Runtime library cannot be in an isolated container!"}, errors.New("compile: emit failure")
		}
	}

	if len(checkDiags) > 0 {
		return nil, checkDiags, errors.New("compile: project configuration diagnostics")
	}

	return &projectContext{
		dir:         dir,
		projectType: projectType,
		rojoContext: &transformer.RojoContext{
			Resolver:          rojoResolver,
			PathTranslator:    pathTranslator,
			RuntimeLibRbxPath: runtimeLibRbxPath,
			ProjectPath:       filepath.FromSlash(dir),
		},
	}, nil, nil
}

// CompileProject compiles every file of the project rooted at projectDir —
// the Go analog of upstream compileFiles.ts: ONE Program, the Rojo context
// computed once, then per file: pre-emit diagnostics -> TransformState ->
// transformSourceFile -> render. The result maps project-relative output
// paths (slash-separated, e.g. "out/main.luau") to rendered Luau text.
// Like CompileFile, any diagnostics fail the compile: text map nil,
// diagnostics returned as strings alongside a hard error.
func CompileProject(projectDir string) (map[string]string, []string, error) {
	dir, program, diags, err := newProjectProgram(projectDir)
	if err != nil {
		return nil, diags, err
	}
	pctx, diags, err := newProjectContext(dir, program)
	if err != nil {
		return nil, diags, err
	}
	ctx := context.Background()

	// Program-level option diagnostics fail the compile before any file is
	// transformed, mirroring CompileFile.
	if tsDiags := program.GetProgramDiagnostics(); len(tsDiags) > 0 {
		return nil, diagnosticStrings(tsDiags), errors.New("compile: TypeScript diagnostics")
	}

	chk, release := program.GetTypeChecker(ctx)
	defer release()
	multi := transformer.NewMultiState()

	results := make(map[string]string)
	for _, sourceFile := range program.SourceFiles() {
		// Upstream compiles the root source files (getChangedSourceFiles
		// filters declaration and JSON files; node_modules/lib files are
		// declaration files and drop out the same way).
		fileName := sourceFile.FileName()
		if sourceFile.IsDeclarationFile ||
			(!strings.HasSuffix(fileName, ".ts") && !strings.HasSuffix(fileName, ".tsx")) {
			continue
		}

		// Per-file pre-emit diagnostics; the first file with errors aborts
		// the pass (compileFiles.ts L156-158 + the hasErrors early return).
		// Running GetSemanticDiagnostics before transforming also populates
		// the checker's alias-reference marks for this file (digest §7.3).
		if tsDiags := program.GetSemanticDiagnostics(ctx, sourceFile); len(tsDiags) > 0 {
			return nil, diagnosticStrings(tsDiags), errors.New("compile: TypeScript diagnostics")
		}

		state := transformer.NewState(program, chk, sourceFile, transformer.NewDiagService(), multi)
		state.SetRojoContext(pctx.rojoContext, pctx.projectType)

		text, fileDiags, err := transformAndRender(state)
		if err != nil {
			return nil, nil, err
		}
		if len(fileDiags) > 0 {
			return nil, fileDiags, errors.New("compile: transformer diagnostics")
		}

		outPath := pctx.rojoContext.PathTranslator.GetOutputPath(fileName)
		relOut, err := filepath.Rel(filepath.FromSlash(dir), outPath)
		if err != nil {
			relOut = outPath
		}
		results[filepath.ToSlash(relOut)] = text
	}

	return results, nil, nil
}

// createPathTranslator ports Project/functions/createPathTranslator.ts:
// rootDir is the common ancestor of the program's common source directory and
// the configured rootDir(s); the buildInfo path is irrelevant to translation
// (the translator stores but never consults it) and rotor always emits the
// .luau extension (DEFAULT_PROJECT_OPTIONS.luau = true).
func createPathTranslator(program *compiler.Program) *rojo.PathTranslator {
	options := program.Options()
	dirs := append([]string{program.CommonSourceDirectory()}, getRootDirs(program)...)
	rootDir := findAncestorDir(dirs)
	outDir := filepath.FromSlash(options.OutDir)
	return rojo.NewPathTranslator(rootDir, outDir, "", options.Declaration.IsTrue(), true)
}

// getRootDirs ports Shared/util/getRootDirs.ts: rootDir if set, else rootDirs
// (the assert is upstream's; tsconfig validation has already run).
func getRootDirs(program *compiler.Program) []string {
	options := program.Options()
	if options.RootDir != "" {
		return []string{options.RootDir}
	}
	if len(options.RootDirs) > 0 {
		return options.RootDirs
	}
	panic("compile: getRootDirs: neither rootDir nor rootDirs is set") // upstream assert
}

// findAncestorDir ports Shared/util/findAncestorDir.ts: the deepest directory
// containing every input directory.
func findAncestorDir(dirs []string) string {
	sep := string(filepath.Separator)
	normalized := make([]string, len(dirs))
	for i, dir := range dirs {
		dir = filepath.Clean(filepath.FromSlash(dir))
		if !strings.HasSuffix(dir, sep) {
			dir += sep
		}
		normalized[i] = dir
	}
	currentDir := normalized[0]
	for !allHavePrefix(normalized, currentDir) {
		currentDir = filepath.Join(currentDir, "..") + sep
	}
	return filepath.Clean(currentDir)
}

func allHavePrefix(dirs []string, prefix string) bool {
	for _, dir := range dirs {
		if !strings.HasPrefix(dir, prefix) {
			return false
		}
	}
	return true
}

// findPackageJSON walks up from dir looking for package.json (the
// ts.findPackageJson call in createProjectData.ts L16). Returns "" when no
// ancestor has one.
func findPackageJSON(dir string) string {
	current := filepath.Clean(filepath.FromSlash(dir))
	for {
		candidate := filepath.Join(current, "package.json")
		if st, err := os.Stat(candidate); err == nil && st.Mode().IsRegular() {
			return candidate
		}
		parent := filepath.Dir(current)
		if parent == current {
			return ""
		}
		current = parent
	}
}

// checkFileName ports Project/functions/checkFileName.ts; returns the
// incorrectFileName diagnostic message or "".
func checkFileName(filePath string) string {
	baseName := filepath.Base(filePath)
	if nameWarning, ok := filenameWarnings[baseName]; ok {
		return transformer.DiagIncorrectFileName(baseName, nameWarning, filePath).Message
	}
	return ""
}

// checkRojoConfig ports Project/functions/checkRojoConfig.ts: a Rojo $path
// partition pointing INSIDE a TypeScript root dir means the user mapped src
// instead of out.
func checkRojoConfig(rojoConfigPath string, resolver *rojo.RojoResolver, rootDirs []string, pathTranslator *rojo.PathTranslator) []string {
	if rojoConfigPath == "" {
		return nil
	}
	var messages []string
	rojoConfigDir := filepath.Dir(rojoConfigPath)
	for _, partition := range resolver.GetPartitions() {
		for _, rootDir := range rootDirs {
			if isPathDescendantOf(partition.FsPath, filepath.FromSlash(rootDir)) {
				outPath := pathTranslator.GetOutputPath(partition.FsPath)
				inputPath := relOrSelf(rojoConfigDir, partition.FsPath)
				suggestedPath := relOrSelf(rojoConfigDir, outPath)
				messages = append(messages, transformer.DiagRojoPathInSrc(inputPath, suggestedPath).Message)
			}
		}
	}
	return messages
}

// isPathDescendantOf mirrors Shared/util/isPathDescendantOf.ts (same quirk as
// the rojo package's private copy).
func isPathDescendantOf(filePath, dirPath string) bool {
	if dirPath == filePath {
		return true
	}
	rel, err := filepath.Rel(dirPath, filePath)
	if err != nil {
		return false
	}
	return !strings.HasPrefix(rel, "..")
}

func relOrSelf(base, target string) string {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return target
	}
	return rel
}
