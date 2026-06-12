package pack

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Format is the artifact rotor pack produces.
type Format string

const (
	FormatLuau  Format = "luau"  // self-reconstructing single Luau script
	FormatRbxmx Format = "rbxmx" // Roblox XML model (from rojo build)
	FormatRbxm  Format = "rbxm"  // Roblox binary model (from rojo build)
)

// Options configures Pack.
type Options struct {
	Project string // a *.project.json file, or a directory containing one
	Format  Format
	Entry   string // luau only: dotted instance path; bundle returns require(entry)
}

// Pack builds the artifact bytes. rbxm/rbxmx are produced by `rojo build` directly;
// luau parses a rojo-built .rbxmx into a self-reconstructing bundle. `rojo` must be
// on PATH (it constructs the authoritative instance tree).
func Pack(opts Options) ([]byte, error) {
	rojoBin, err := exec.LookPath("rojo")
	if err != nil {
		return nil, fmt.Errorf("rojo is required but not on PATH — rotor pack builds the instance tree with `rojo build`")
	}
	project, err := resolveProject(opts.Project)
	if err != nil {
		return nil, err
	}

	// For luau we still build a .rbxmx (XML) to read the tree from.
	buildExt := string(opts.Format)
	if opts.Format == FormatLuau {
		buildExt = "rbxmx"
	}

	tmp, err := os.CreateTemp("", "rotor-pack-*."+buildExt)
	if err != nil {
		return nil, err
	}
	tmpPath := tmp.Name()
	_ = tmp.Close()
	defer os.Remove(tmpPath)

	cmd := exec.Command(rojoBin, "build", project, "-o", tmpPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("rojo build failed: %v\n%s", err, out)
	}
	data, err := os.ReadFile(tmpPath)
	if err != nil {
		return nil, err
	}

	if opts.Format != FormatLuau {
		return data, nil // rojo-built model bytes
	}

	roots, err := ParseRbxmx(data)
	if err != nil {
		return nil, fmt.Errorf("parsing rojo model: %w", err)
	}
	src, err := EmitLuau(roots, opts.Entry)
	if err != nil {
		return nil, err
	}
	return []byte(src), nil
}

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
