// Package dotenv loads the compile-time environment consumed by rotor's
// $env macro (a built-in replacement for the community rbxts-transform-env
// transformer plugin).
//
// Lookup priority, highest first:
//  1. the real process environment,
//  2. `.env.<NODE_ENV>` in the project directory,
//  3. `.env` in the project directory.
//
// NODE_ENV itself resolves from the process environment first, then from the
// `.env` file. Only plain string values are supported (v1): KEY=VALUE lines,
// full-line `#` comments, optional matching single or double quotes stripped
// from the value, no multiline values, no escapes, no variable expansion.
package dotenv

import (
	"os"
	"path/filepath"
	"strings"
)

// Env is one project's compile-time environment snapshot. The file-derived
// values are read once per Load call; the process environment is consulted
// live at Lookup time (it cannot change between Setenv-free goroutines of a
// single compile pass, so a pass stays internally consistent).
//
// Determinism/caching: rotor loads a fresh Env on every compile pass
// (newProjectContext runs once per CompileProject/CompileFile/Build pass), so
// watch rebuilds pick up .env edits on the next build. NOTE the incremental
// build manifest hashes source files only — a changed .env value does not by
// itself re-select files that inlined the old value; use a non-incremental
// (clean) build after editing .env, or touch the affected sources.
type Env struct {
	fileVars map[string]string
}

// Load reads `.env` and `.env.<NODE_ENV>` from dir (the directory containing
// the project's tsconfig). Missing files are fine; an Env with no file
// values still resolves process-environment variables.
func Load(dir string) *Env {
	fileVars := make(map[string]string)

	base := readEnvFile(filepath.Join(dir, ".env"))
	for k, v := range base {
		fileVars[k] = v
	}

	// NODE_ENV: process environment first, then the .env file itself.
	nodeEnv, ok := os.LookupEnv("NODE_ENV")
	if !ok || nodeEnv == "" {
		nodeEnv = base["NODE_ENV"]
	}
	if nodeEnv != "" {
		// .env.<NODE_ENV> values override .env values.
		for k, v := range readEnvFile(filepath.Join(dir, ".env."+nodeEnv)) {
			fileVars[k] = v
		}
	}

	return &Env{fileVars: fileVars}
}

// Lookup resolves name with the documented priority: process environment,
// then the merged .env file values. ok is false only when name is defined
// nowhere. Safe on a nil receiver (process environment only), so transformer
// states constructed without a project env still behave sensibly.
func (e *Env) Lookup(name string) (value string, ok bool) {
	if value, ok = os.LookupEnv(name); ok {
		return value, true
	}
	if e == nil {
		return "", false
	}
	value, ok = e.fileVars[name]
	return value, ok
}

// readEnvFile parses path with Parse; a missing or unreadable file yields nil.
func readEnvFile(path string) map[string]string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return Parse(string(data))
}

// Parse parses .env text: one KEY=VALUE per line, CRLF tolerated, blank
// lines and full-line `#` comments skipped, surrounding whitespace trimmed
// from key and value, one level of matching single or double quotes stripped
// from the value (whitespace inside quotes is preserved). Lines without `=`
// and lines with an empty key are ignored. Later duplicates win.
func Parse(src string) map[string]string {
	vars := make(map[string]string)
	for _, line := range strings.Split(src, "\n") {
		line = strings.TrimRight(line, "\r")
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		if key == "" {
			continue
		}
		value := strings.TrimSpace(line[eq+1:])
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}
		vars[key] = value
	}
	return vars
}
