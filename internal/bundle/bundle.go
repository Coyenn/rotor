// Package bundle inlines a Luau require graph into a single runnable file, in the
// style of darklua's path-require bundler. require("...") calls that resolve to a
// local file are rewritten to a memoized module loader; requires that don't resolve
// to a file (e.g. Roblox instance-path or service requires) are left untouched.
package bundle

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"rotor/internal/luau/cst"
)

// modulesIdentifier is the global table that holds every bundled module's loader and
// cache. It is unlikely to collide with user code.
const modulesIdentifier = "__ROTOR_BUNDLE"

type module struct {
	path    string // absolute file path
	id      int
	file    *cst.File
	replace map[cst.Node]string // require-call node -> loader call text
}

// Bundle resolves the require graph rooted at entryPath and returns a single Luau
// source string that, when run, behaves like the original entry module (run-once
// caching per module; a genuinely cyclic require errors at runtime like a Roblox
// ModuleScript would).
func Bundle(entryPath string) (string, error) {
	entryAbs, err := filepath.Abs(entryPath)
	if err != nil {
		return "", err
	}

	var modules []*module
	byPath := map[string]*module{}

	var process func(absPath string) (*module, error)
	process = func(absPath string) (*module, error) {
		if m, ok := byPath[absPath]; ok {
			return m, nil
		}
		src, err := os.ReadFile(absPath)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", absPath, err)
		}
		file, diags := cst.Parse(string(src))
		if len(diags) != 0 {
			d := diags[0]
			return nil, fmt.Errorf("%s:%d:%d: %s", absPath, d.Pos.Line, d.Pos.Col, d.Message)
		}
		m := &module{path: absPath, id: len(modules), file: file, replace: map[cst.Node]string{}}
		byPath[absPath] = m // register before recursing so cycles terminate
		modules = append(modules, m)

		var walkErr error
		cst.EachNode(file, func(n cst.Node) {
			if walkErr != nil {
				return
			}
			call, ok := n.(*cst.Call)
			if !ok {
				return
			}
			reqPath, ok := requireTarget(call)
			if !ok {
				return
			}
			resolved, ok := resolveRequire(absPath, reqPath)
			if !ok {
				return // unresolved -> leave as a runtime require
			}
			dep, err := process(resolved)
			if err != nil {
				walkErr = err
				return
			}
			m.replace[call] = fmt.Sprintf("%s.load_%d()", modulesIdentifier, dep.id)
		})
		if walkErr != nil {
			return nil, walkErr
		}
		return m, nil
	}

	entry, err := process(entryAbs)
	if err != nil {
		return "", err
	}
	return assemble(modules, entry), nil
}

// requireTarget reports whether call is require("<literal>") (or require"<literal>")
// and returns the decoded literal path.
func requireTarget(call *cst.Call) (string, bool) {
	name, ok := call.Base.(*cst.Name)
	if !ok || name.Tok.Token.Text != "require" {
		return "", false
	}
	switch a := call.Args.(type) {
	case *cst.ParenArgs:
		if len(a.List.Items) == 1 {
			if s, ok := a.List.Items[0].(*cst.String); ok {
				return cst.StringValue(s)
			}
		}
	case *cst.String:
		return cst.StringValue(a)
	}
	return "", false
}

// resolveRequire resolves a require path (relative to the requiring file) to an
// existing Luau file, trying the Rojo/Luau extension and init conventions.
func resolveRequire(fromFile, reqPath string) (string, bool) {
	if reqPath == "" {
		return "", false
	}
	base := filepath.Join(filepath.Dir(fromFile), reqPath)
	for _, c := range []string{
		base + ".luau",
		base + ".lua",
		filepath.Join(base, "init.luau"),
		filepath.Join(base, "init.lua"),
		base,
	} {
		if isFile(c) {
			return c, true
		}
	}
	return "", false
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// assemble emits the bundle: the module table, one do-block per module (an impl
// closure + its memoizing loader), and a final call to the entry's loader.
func assemble(modules []*module, entry *module) string {
	var b strings.Builder
	fmt.Fprintf(&b, "local %s = { cache = {}, loading = {} }\n", modulesIdentifier)
	for _, m := range modules {
		body := cst.UnparseWith(m.file, m.replace)
		fmt.Fprintf(&b, "do\n")
		fmt.Fprintf(&b, "\tlocal function impl_%d(...)\n", m.id)
		b.WriteString(body)
		if !strings.HasSuffix(body, "\n") {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "\tend\n")
		writeLoader(&b, m.id)
		fmt.Fprintf(&b, "end\n")
	}
	fmt.Fprintf(&b, "return %s.load_%d()\n", modulesIdentifier, entry.id)
	return b.String()
}

func writeLoader(b *strings.Builder, id int) {
	t := modulesIdentifier
	fmt.Fprintf(b, "\tfunction %s.load_%d()\n", t, id)
	fmt.Fprintf(b, "\t\tlocal cached = %s.cache[%d]\n", t, id)
	fmt.Fprintf(b, "\t\tif cached ~= nil then return cached.value end\n")
	fmt.Fprintf(b, "\t\tif %s.loading[%d] then error(\"Requested module was required recursively\", 2) end\n", t, id)
	fmt.Fprintf(b, "\t\t%s.loading[%d] = true\n", t, id)
	fmt.Fprintf(b, "\t\tlocal value = impl_%d()\n", id)
	fmt.Fprintf(b, "\t\t%s.loading[%d] = nil\n", t, id)
	fmt.Fprintf(b, "\t\t%s.cache[%d] = { value = value }\n", t, id)
	fmt.Fprintf(b, "\t\treturn value\n")
	fmt.Fprintf(b, "\tend\n")
}
