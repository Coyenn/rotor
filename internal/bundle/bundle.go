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

// Options tunes a bundle run.
type Options struct {
	// Exclude is a list of doublestar globs; a require whose resolved file path
	// matches any of them is left verbatim (not inlined), for modules provided at
	// runtime. Globs are matched against the resolved absolute path and, for
	// robustness across platforms, also against the path relative to the bundle
	// root (the entry file's directory).
	Exclude []string
}

// Bundle resolves the require graph rooted at entryPath with default options.
func Bundle(entryPath string) (string, error) {
	return BundleWith(entryPath, Options{})
}

// BundleWith resolves the require graph rooted at entryPath and returns a single
// Luau source string that, when run, behaves like the original entry module
// (run-once caching per module; a genuinely cyclic require errors at runtime like
// a Roblox ModuleScript would).
//
// Beyond plain code modules it also handles: .luaurc "@alias" requires (resolved
// through the nearest .luaurc), data-file requires (.json -> Luau table, .txt/.md
// -> Luau string, each a cached module), and --exclude globs (matching requires
// left verbatim).
func BundleWith(entryPath string, opts Options) (string, error) {
	entryAbs, err := filepath.Abs(entryPath)
	if err != nil {
		return "", err
	}
	// The bundle root is the entry file's directory: the ceiling for .luaurc
	// walk-up and the base for relative exclude-glob matching.
	root := filepath.Dir(entryAbs)
	aliases := newAliasResolver(root)

	var modules []*module
	byPath := map[string]*module{}

	// excluded reports whether a resolved path matches any --exclude glob.
	excluded := func(absPath string) bool {
		if len(opts.Exclude) == 0 {
			return false
		}
		rel, err := filepath.Rel(root, absPath)
		for _, g := range opts.Exclude {
			if matchGlob(g, absPath) {
				return true
			}
			if err == nil && matchGlob(g, rel) {
				return true
			}
		}
		return false
	}

	// newModule registers a parsed file as a module. src must already parse.
	newModule := func(absPath, src string) (*module, error) {
		file, diags := cst.Parse(src)
		if len(diags) != 0 {
			d := diags[0]
			return nil, fmt.Errorf("%s:%d:%d: %s", absPath, d.Pos.Line, d.Pos.Col, d.Message)
		}
		m := &module{path: absPath, id: len(modules), file: file, replace: map[cst.Node]string{}}
		byPath[absPath] = m // register before recursing so cycles terminate
		modules = append(modules, m)
		return m, nil
	}

	var process func(absPath string) (*module, error)
	process = func(absPath string) (*module, error) {
		if m, ok := byPath[absPath]; ok {
			return m, nil
		}
		src, err := os.ReadFile(absPath)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", absPath, err)
		}
		// Data files become synthetic `return <literal>` modules: no requires to
		// walk, but otherwise an ordinary cached module.
		if isDataFile(absPath) {
			modSrc, err := dataModuleSource(absPath, src)
			if err != nil {
				return nil, err
			}
			return newModule(absPath, modSrc)
		}
		m, err := newModule(absPath, string(src))
		if err != nil {
			return nil, err
		}

		var walkErr error
		cst.EachNode(m.file, func(n cst.Node) {
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
			resolved, ok := resolve(aliases, absPath, reqPath)
			if !ok {
				return // unresolved -> leave as a runtime require
			}
			if excluded(resolved) {
				return // excluded -> leave as a runtime require
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

// resolve maps a require path to an existing file. "@alias/..." requires are
// expanded through the nearest .luaurc first; everything else (and the expanded
// alias base) goes through resolveBase's extension/init conventions. Unresolved
// requires (instance paths, missing files, unknown aliases) report ok == false
// and are left verbatim by the caller.
func resolve(aliases *aliasResolver, fromFile, reqPath string) (string, bool) {
	if reqPath == "" {
		return "", false
	}
	if base, ok := aliases.resolveAlias(fromFile, reqPath); ok {
		return resolveBase(base)
	}
	if strings.HasPrefix(reqPath, "@") {
		return "", false // alias-shaped but undefined: leave verbatim
	}
	return resolveBase(filepath.Join(filepath.Dir(fromFile), reqPath))
}

// resolveBase resolves an absolute base path to an existing file, trying the
// Rojo/Luau extension and init conventions plus the verbatim path (so a require
// that already names a file, e.g. "./data.json", resolves directly).
func resolveBase(base string) (string, bool) {
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
