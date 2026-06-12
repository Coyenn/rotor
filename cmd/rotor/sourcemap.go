package main

import (
	"fmt"
	"os"
	"strings"

	"rotor/internal/sourcemap"
)

// cmdSourcemap emits a Rojo-compatible sourcemap.json for the project — the
// format `rojo sourcemap --include-non-scripts` produces, which luau-lsp
// consumes. The tree is built natively (no rojo) for plain script trees;
// projects using features outside that subset fall back to `rojo sourcemap`
// when rojo is on PATH. File paths are project-relative with forward slashes.
// Output goes to --output, or to stdout when no output path is given.
func cmdSourcemap(args []string) int {
	project := ""
	output := ""
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-h" || a == "--help":
			usage(os.Stdout)
			return 0
		case a == "-o" || a == "--output":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "rotor sourcemap: %s requires a path\n", a)
				return 1
			}
			i++
			output = args[i]
		case strings.HasPrefix(a, "--output="):
			output = strings.TrimPrefix(a, "--output=")
		case strings.HasPrefix(a, "-o="):
			output = strings.TrimPrefix(a, "-o=")
		case strings.HasPrefix(a, "-"):
			fmt.Fprintf(os.Stderr, "rotor sourcemap: unknown flag %q\n\n", a)
			usage(os.Stderr)
			return 1
		default:
			if project != "" {
				fmt.Fprintf(os.Stderr, "rotor sourcemap: unexpected extra argument %q\n\n", a)
				usage(os.Stderr)
				return 1
			}
			project = a
		}
	}

	data, err := sourcemap.Generate(project)
	if err != nil {
		fmt.Fprintf(os.Stderr, "rotor sourcemap: %v\n", err)
		return 1
	}
	if output == "" {
		_, _ = os.Stdout.Write(data)
		return 0
	}
	if err := os.WriteFile(output, data, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "rotor sourcemap: cannot write %q: %v\n", output, err)
		return 1
	}
	return 0
}
