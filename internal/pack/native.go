package pack

import (
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"rotor/internal/rojo"
)

// BuildNative constructs the instance tree directly from a Rojo project file and the
// filesystem, without invoking `rojo`. It reproduces Rojo's tree for the common
// script-tree subset — Folder/Model roots, $path-mapped directories of .luau/.lua
// files (with .server/.client sub-extensions and init directories), and .txt ->
// StringValue. Anything it cannot reproduce 1:1 (DataModel/services, .json/.toml/
// .model.json/.meta.json/.csv/.rbxm files, nested projects, bare containers needing
// service-name inference) sets supported=false, signalling the caller to fall back
// to `rojo build`.
//
// Because the --as luau bundle only models Name/ClassName/Source/Value, a native
// tree over the supported subset yields a byte-identical bundle to the rojo path
// (verified by TestNativeMatchesRojo).
func BuildNative(projectFile string) (root *Instance, supported bool, err error) {
	name, tree, err := rojo.ParseProjectFile(projectFile)
	if err != nil {
		return nil, false, err
	}
	if name == "" {
		name = "Model"
	}
	b := &nativeBuilder{dir: filepath.Dir(projectFile), supported: true}
	root = b.fromTree(name, tree)
	return root, b.supported, nil
}

type nativeBuilder struct {
	dir       string
	supported bool
}

func (b *nativeBuilder) unsupportedf() { b.supported = false }

// fromTree builds an instance named name from a Rojo tree node.
func (b *nativeBuilder) fromTree(name string, tree *rojo.Tree) *Instance {
	inst := &Instance{Name: name, ClassName: tree.ClassName}

	// A DataModel (game) root needs service-name inference we don't do 1:1.
	if tree.ClassName == "DataModel" {
		b.unsupportedf()
	}

	if tree.Path != nil {
		b.applyPath(inst, filepath.Join(b.dir, *tree.Path))
	}

	for _, child := range tree.Children {
		inst.Children = append(inst.Children, b.fromTree(child.Name, child.Tree))
	}

	if inst.ClassName == "" {
		if tree.Path == nil && len(tree.Children) == 0 {
			// A bare node with neither class, path, nor children is ambiguous
			// (could be a service); let rojo decide.
			b.unsupportedf()
		}
		inst.ClassName = "Folder"
	}
	return inst
}

// applyPath sets an instance's class/source/value/children from a filesystem path,
// without changing its Name.
func (b *nativeBuilder) applyPath(inst *Instance, fsPath string) {
	info, err := os.Stat(fsPath)
	if err != nil {
		b.unsupportedf()
		return
	}
	if !info.IsDir() {
		cn, src, val, ok := classifyContent(fsPath)
		if !ok {
			b.unsupportedf()
			return
		}
		if inst.ClassName == "" {
			inst.ClassName = cn
		}
		inst.Source = src
		inst.Value = val
		return
	}
	b.applyDir(inst, fsPath)
}

// applyDir handles a directory: an init script (making the directory a script) or a
// Folder, plus the directory's entries as children (sorted by name).
func (b *nativeBuilder) applyDir(inst *Instance, dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		b.unsupportedf()
		return
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)

	initFile, initClass := findInit(names)
	if initFile != "" {
		data, err := os.ReadFile(filepath.Join(dir, initFile))
		if err != nil {
			b.unsupportedf()
			return
		}
		if inst.ClassName == "" {
			inst.ClassName = initClass
		}
		inst.Source = string(data)
	} else if inst.ClassName == "" {
		inst.ClassName = "Folder"
	}

	for _, name := range names {
		if name == initFile || isIgnored(name) {
			continue
		}
		if strings.HasSuffix(strings.ToLower(name), ".meta.json") {
			b.unsupportedf() // property injection we don't model
			continue
		}
		if name == "default.project.json" {
			b.unsupportedf() // nested project
			continue
		}
		child := b.fromDirEntry(filepath.Join(dir, name))
		if child != nil {
			inst.Children = append(inst.Children, child)
		}
	}
}

// fromDirEntry builds an instance from a filesystem entry, deriving its name.
func (b *nativeBuilder) fromDirEntry(fsPath string) *Instance {
	info, err := os.Stat(fsPath)
	if err != nil {
		b.unsupportedf()
		return nil
	}
	if info.IsDir() {
		inst := &Instance{Name: filepath.Base(fsPath)}
		b.applyDir(inst, fsPath)
		return inst
	}
	name, cn, src, val, ok := classifyFile(fsPath)
	if !ok {
		b.unsupportedf()
		return nil
	}
	return &Instance{Name: name, ClassName: cn, Source: src, Value: val}
}

// findInit returns the init script file name in a directory (if any) and its class.
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

// classifyContent classifies a file by extension into (className, source, value),
// without deriving a name. ok is false for unsupported file types.
func classifyContent(fsPath string) (className, source, value string, ok bool) {
	data, err := os.ReadFile(fsPath)
	if err != nil {
		return "", "", "", false
	}
	switch cn := scriptClass(fsPath); {
	case cn != "":
		return cn, string(data), "", true
	case strings.HasSuffix(strings.ToLower(fsPath), ".txt"):
		return "StringValue", "", string(data), true
	default:
		return "", "", "", false
	}
}

// classifyFile is classifyContent plus the Rojo name derivation (extension and
// .server/.client sub-extension stripped).
func classifyFile(fsPath string) (name, className, source, value string, ok bool) {
	cn, src, val, ok := classifyContent(fsPath)
	if !ok {
		return "", "", "", "", false
	}
	return deriveName(filepath.Base(fsPath)), cn, src, val, true
}

// scriptClass returns the script class for a .luau/.lua path (honoring .server /
// .client sub-extensions), or "" if the file is not a script.
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

// deriveName strips the recognized extension and sub-extension from a base file name,
// preserving the original casing of the remaining stem.
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
