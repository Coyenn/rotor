package bundle

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// luaurc is the subset of a .luaurc we care about: the alias map. A real .luaurc
// is JSON5-ish (it may carry comments, trailing commas, and unquoted keys);
// encoding/json only handles the strict-JSON subset. That subset covers the
// common, formatter-emitted .luaurc, which is what rotor projects ship. When a
// .luaurc fails to parse as strict JSON we warn and skip its aliases rather than
// aborting the bundle (see loadLuaurc) — full JSON5 support is intentionally out
// of scope here.
type luaurc struct {
	Aliases map[string]string `json:"aliases"`
}

// aliasResolver caches .luaurc lookups per directory while walking the require
// graph, and remembers which directories were already warned about so a broken
// .luaurc is reported at most once. The zero value is ready to use, but callers
// should construct one via newAliasResolver so the maps are initialized.
type aliasResolver struct {
	root  string             // bundle root: walk-up stops here (inclusive)
	cache map[string]*luaurc // dir -> parsed .luaurc (nil when absent/broken)
	warns map[string]bool    // dir -> already warned about a broken .luaurc
	warn  func(format string, a ...any)
}

func newAliasResolver(root string) *aliasResolver {
	return &aliasResolver{
		root:  root,
		cache: map[string]*luaurc{},
		warns: map[string]bool{},
		warn: func(format string, a ...any) {
			fmt.Fprintf(os.Stderr, "rotor bundle: "+format+"\n", a...)
		},
	}
}

// resolveAlias resolves an "@alias/sub/path" require relative to fromFile by
// finding the nearest .luaurc (walking up from fromFile's directory to the
// bundle root) whose aliases map contains the alias. It returns the on-disk
// path the alias prefix expands to (alias dir joined with the remainder), still
// to be run through the normal .luau/.lua/init resolution by the caller. ok is
// false when the require is not an "@" alias or no .luaurc defines it.
func (r *aliasResolver) resolveAlias(fromFile, reqPath string) (string, bool) {
	if !strings.HasPrefix(reqPath, "@") {
		return "", false
	}
	// Split "@alias/rest" into the alias name (sans "@") and the remainder.
	name := strings.TrimPrefix(reqPath, "@")
	rest := ""
	if i := strings.IndexByte(name, '/'); i >= 0 {
		name, rest = name[:i], name[i+1:]
	}
	if name == "" {
		return "", false
	}

	dir := filepath.Dir(fromFile)
	for {
		rc := r.load(dir)
		if rc != nil {
			if target, ok := rc.Aliases[name]; ok && target != "" {
				// Alias targets are resolved relative to the .luaurc that
				// declares them (the Luau convention), then joined with the
				// remaining path segments.
				base := target
				if !filepath.IsAbs(base) {
					base = filepath.Join(dir, filepath.FromSlash(target))
				}
				if rest != "" {
					base = filepath.Join(base, filepath.FromSlash(rest))
				}
				return base, true
			}
		}
		if pathsEqual(dir, r.root) {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir { // reached filesystem root without hitting bundle root
			break
		}
		dir = parent
	}
	return "", false
}

// load reads and parses dir/.luaurc, caching the result (nil for absent or
// broken). A broken .luaurc warns once per directory and is treated as absent.
func (r *aliasResolver) load(dir string) *luaurc {
	if rc, ok := r.cache[dir]; ok {
		return rc
	}
	p := filepath.Join(dir, ".luaurc")
	data, err := os.ReadFile(p)
	if err != nil {
		r.cache[dir] = nil
		return nil
	}
	var rc luaurc
	if err := json.Unmarshal(data, &rc); err != nil {
		if !r.warns[dir] {
			r.warns[dir] = true
			r.warn("ignoring %s: not strict JSON (%v); aliases from it are skipped", p, err)
		}
		r.cache[dir] = nil
		return nil
	}
	r.cache[dir] = &rc
	return &rc
}

// pathsEqual compares two paths after cleaning, case-insensitively (matching the
// case-insensitive filesystems rotor projects typically use).
func pathsEqual(a, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}
