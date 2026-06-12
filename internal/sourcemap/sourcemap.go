// Package sourcemap generates Rojo-compatible sourcemap.json files — the
// format `rojo sourcemap --include-non-scripts` emits and luau-lsp consumes —
// directly from a Rojo project file, without running rojo for the common
// script-tree subset.
//
// Output parity was verified against Rojo 7.6.1: compact JSON with key order
// name/className/filePaths/children, empty filePaths/children omitted, a
// trailing newline, and file paths relative to the project file's directory
// with forward slashes. Child ordering matches rojo: children synced from a
// $path directory come first (name-sorted), then project-declared children in
// byte-sorted order (rojo keeps project node children in a BTreeMap). Only the
// root node carries the project file in filePaths, appended after any
// $path-derived files.
//
// Projects using features outside the native subset (globIgnorePaths,
// .json/.toml/.csv/.model.json/.meta.json/.rbxm/.rbxmx files, nested projects,
// or $path entries missing from disk) fall back to running `rojo sourcemap`
// when rojo is on PATH, mirroring internal/pack.
package sourcemap

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"rotor/internal/rojo"
)

// Node is one instance in a Rojo sourcemap. Field order and the omitempty
// rules mirror rojo's serialization exactly.
type Node struct {
	Name      string   `json:"name"`
	ClassName string   `json:"className"`
	FilePaths []string `json:"filePaths,omitempty"`
	Children  []*Node  `json:"children,omitempty"`
}

// Generate produces sourcemap bytes for a project (a *.project.json file, or a
// directory containing one; "" means "."). It builds the tree natively when
// the project is within the supported subset, falling back to
// `rojo sourcemap --include-non-scripts` otherwise.
func Generate(project string) ([]byte, error) {
	projectFile, err := resolveProject(project)
	if err != nil {
		return nil, err
	}
	root, supported, err := Build(projectFile)
	if err != nil {
		return nil, err
	}
	if !supported {
		return generateViaRojo(projectFile)
	}
	return Marshal(root)
}

// Marshal encodes a sourcemap tree the way rojo does: compact JSON, no HTML
// escaping, trailing newline.
func Marshal(root *Node) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(root); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Build constructs the sourcemap tree natively from a Rojo project file.
// supported reports whether the project is within the native subset; when it
// is false the returned tree is incomplete and the caller should fall back to
// `rojo sourcemap`.
func Build(projectFile string) (root *Node, supported bool, err error) {
	name, tree, err := rojo.ParseProjectFile(projectFile)
	if err != nil {
		return nil, false, err
	}
	b := &builder{base: filepath.Dir(projectFile), supported: true}
	if hasGlobIgnorePaths(projectFile) {
		// rojo filters synced files through these globs; reproduce via rojo.
		b.supported = false
	}
	root = b.fromTree(name, tree)
	// Only the root carries the project file, appended after $path files.
	root.FilePaths = append(root.FilePaths, b.rel(projectFile))
	return root, b.supported, nil
}

// hasGlobIgnorePaths reports whether the project file declares a non-empty
// globIgnorePaths array (a feature the native builder does not model).
func hasGlobIgnorePaths(projectFile string) bool {
	data, err := os.ReadFile(projectFile)
	if err != nil {
		return true // be conservative; rojo will surface the real error
	}
	var meta struct {
		GlobIgnorePaths []string `json:"globIgnorePaths"`
	}
	if json.Unmarshal(data, &meta) != nil {
		return false
	}
	return len(meta.GlobIgnorePaths) > 0
}

type builder struct {
	base      string
	supported bool
}

func (b *builder) unsupported() { b.supported = false }

// rel converts an absolute-or-project-joined path into the project-relative,
// forward-slash form rojo emits.
func (b *builder) rel(fsPath string) string {
	r, err := filepath.Rel(b.base, fsPath)
	if err != nil {
		return filepath.ToSlash(fsPath)
	}
	return filepath.ToSlash(r)
}

// fromTree builds a node named name from a Rojo project tree node.
func (b *builder) fromTree(name string, tree *rojo.Tree) *Node {
	n := &Node{Name: name, ClassName: tree.ClassName}
	if tree.Path != nil {
		b.applyPath(n, filepath.Join(b.base, *tree.Path))
	}
	// Project-declared children come after $path-derived ones, byte-sorted by
	// name (rojo stores project node children in a BTreeMap).
	declared := slices.Clone(tree.Children)
	sort.SliceStable(declared, func(i, j int) bool { return declared[i].Name < declared[j].Name })
	for _, child := range declared {
		n.Children = append(n.Children, b.fromTree(child.Name, child.Tree))
	}
	if n.ClassName == "" {
		if tree.Path == nil {
			// $className is optional when the node's name is itself a class:
			// services under DataModel, StarterPlayerScripts under
			// StarterPlayer, and so on. Rojo errors on names it cannot
			// resolve, so for valid projects the name is the class.
			n.ClassName = name
		} else {
			n.ClassName = "Folder"
		}
	}
	return n
}

// applyPath fills a node's class/filePaths/children from a filesystem path,
// without changing its Name.
func (b *builder) applyPath(n *Node, fsPath string) {
	info, err := os.Stat(fsPath)
	if err != nil {
		// Missing or unreadable $path: possibly optional, possibly not built
		// yet. Let rojo decide what it means.
		b.unsupported()
		return
	}
	if !info.IsDir() {
		cn, ok := classifyClass(fsPath)
		if !ok {
			b.unsupported()
			return
		}
		if n.ClassName == "" {
			n.ClassName = cn
		}
		n.FilePaths = append(n.FilePaths, b.rel(fsPath))
		return
	}
	b.applyDir(n, fsPath)
}

// applyDir handles a directory: an init script (making the directory a script)
// or a Folder, plus the directory's entries as children in name-sorted order.
func (b *builder) applyDir(n *Node, dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		b.unsupported()
		return
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)

	initFile, initClass := findInit(names)
	if initFile != "" {
		if n.ClassName == "" {
			n.ClassName = initClass
		}
		n.FilePaths = append(n.FilePaths, b.rel(filepath.Join(dir, initFile)))
	} else if n.ClassName == "" {
		n.ClassName = "Folder"
	}

	for _, name := range names {
		if name == initFile || isIgnored(name) {
			continue
		}
		full := filepath.Join(dir, name)
		info, err := os.Stat(full)
		if err != nil {
			b.unsupported()
			continue
		}
		if info.IsDir() {
			child := &Node{Name: name}
			b.applyDir(child, full)
			n.Children = append(n.Children, child)
			continue
		}
		lower := strings.ToLower(name)
		switch {
		case strings.HasSuffix(lower, ".project.json"):
			b.unsupported() // nested project
		case strings.HasSuffix(lower, ".meta.json"):
			b.unsupported() // can change sibling/parent classes
		case strings.HasSuffix(lower, ".model.json"),
			strings.HasSuffix(lower, ".json"),
			strings.HasSuffix(lower, ".toml"),
			strings.HasSuffix(lower, ".csv"),
			strings.HasSuffix(lower, ".rbxm"),
			strings.HasSuffix(lower, ".rbxmx"):
			b.unsupported() // rojo middleware the native builder doesn't model
		default:
			cn, ok := classifyClass(full)
			if !ok {
				continue // rojo silently ignores unknown file types
			}
			n.Children = append(n.Children, &Node{
				Name:      deriveName(name),
				ClassName: cn,
				FilePaths: []string{b.rel(full)},
			})
		}
	}
}

// generateViaRojo shells out to `rojo sourcemap --include-non-scripts`, whose
// output (paths relative to the project file's directory, forward slashes,
// trailing newline) matches the native builder's.
func generateViaRojo(projectFile string) ([]byte, error) {
	rojoBin, err := exec.LookPath("rojo")
	if err != nil {
		return nil, fmt.Errorf("this project uses features beyond the native sourcemap subset " +
			"(globIgnorePaths, .json/.toml/.csv/.model.json/.meta.json/.rbxm files, nested projects, " +
			"or $path entries that do not exist yet) and needs `rojo sourcemap`, but rojo is not on PATH")
	}
	cmd := exec.Command(rojoBin, "sourcemap", "--include-non-scripts", projectFile)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("rojo sourcemap failed: %v\n%s", err, stderr.Bytes())
	}
	return out, nil
}

// resolveProject resolves a project argument (file, directory, or "") to a
// *.project.json path, mirroring internal/pack.
func resolveProject(p string) (string, error) {
	if p == "" {
		p = "."
	}
	info, err := os.Stat(p)
	if err != nil {
		return "", fmt.Errorf("project path %q: %w", p, err)
	}
	if !info.IsDir() {
		return p, nil
	}
	if def := filepath.Join(p, "default.project.json"); isFile(def) {
		return def, nil
	}
	matches, _ := filepath.Glob(filepath.Join(p, "*.project.json"))
	if len(matches) > 0 {
		return matches[0], nil
	}
	return "", fmt.Errorf("no *.project.json found in %s", p)
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// findInit returns the init script file name in a directory (if any) and its
// class, with the same precedence internal/pack uses.
func findInit(names []string) (file, class string) {
	for _, candidate := range []struct {
		file, class string
	}{
		{"init.luau", "ModuleScript"},
		{"init.lua", "ModuleScript"},
		{"init.server.luau", "Script"},
		{"init.server.lua", "Script"},
		{"init.client.luau", "LocalScript"},
		{"init.client.lua", "LocalScript"},
	} {
		if slices.Contains(names, candidate.file) {
			return candidate.file, candidate.class
		}
	}
	return "", ""
}

// classifyClass returns the instance class for a file the native subset
// models: .luau/.lua scripts (honoring .server/.client sub-extensions) and
// .txt StringValues. ok is false for anything else.
func classifyClass(fsPath string) (className string, ok bool) {
	if cn := scriptClass(fsPath); cn != "" {
		return cn, true
	}
	if strings.HasSuffix(strings.ToLower(fsPath), ".txt") {
		return "StringValue", true
	}
	return "", false
}

// scriptClass returns the script class for a .luau/.lua path (honoring
// .server/.client sub-extensions), or "" if the file is not a script.
func scriptClass(fsPath string) string {
	lower := strings.ToLower(fsPath)
	stem, ok := trimLuaExt(lower)
	if !ok {
		return ""
	}
	switch {
	case strings.HasSuffix(stem, ".server"):
		return "Script"
	case strings.HasSuffix(stem, ".client"):
		return "LocalScript"
	default:
		return "ModuleScript"
	}
}

func trimLuaExt(lower string) (string, bool) {
	if stem, ok := strings.CutSuffix(lower, ".luau"); ok {
		return stem, true
	}
	if stem, ok := strings.CutSuffix(lower, ".lua"); ok {
		return stem, true
	}
	return "", false
}

// deriveName strips the recognized extension and sub-extension from a base
// file name, preserving the original casing of the remaining stem.
func deriveName(base string) string {
	lower := strings.ToLower(base)
	if stem, ok := trimLuaExt(lower); ok {
		n := len(base) - (len(lower) - len(stem))
		out := base[:n]
		for _, sub := range []string{".server", ".client"} {
			if strings.HasSuffix(strings.ToLower(out), sub) {
				out = out[:len(out)-len(sub)]
				break
			}
		}
		return out
	}
	if strings.HasSuffix(lower, ".txt") {
		return base[:len(base)-len(".txt")]
	}
	return base
}

func isIgnored(name string) bool {
	switch name {
	case ".DS_Store", "Thumbs.db", ".git":
		return true
	}
	return false
}
