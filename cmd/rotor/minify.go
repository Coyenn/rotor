package main

import (
	"fmt"
	"os"
	"strings"

	"rotor/internal/luau/cst"
)

// cmdMinify minifies a single Luau file: it strips comments (except leading `--!`
// directives) and superfluous whitespace, preserving program semantics. Output goes
// to --output, or to stdout when no output path is given.
func cmdMinify(args []string) int {
	input := ""
	output := ""
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-h" || a == "--help":
			usage(os.Stdout)
			return 0
		case a == "-o" || a == "--output":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "rotor minify: %s requires a path\n", a)
				return 1
			}
			i++
			output = args[i]
		case strings.HasPrefix(a, "--output="):
			output = strings.TrimPrefix(a, "--output=")
		case strings.HasPrefix(a, "-o="):
			output = strings.TrimPrefix(a, "-o=")
		case strings.HasPrefix(a, "-"):
			fmt.Fprintf(os.Stderr, "rotor minify: unknown flag %q\n\n", a)
			usage(os.Stderr)
			return 1
		default:
			if input != "" {
				fmt.Fprintf(os.Stderr, "rotor minify: unexpected extra argument %q\n\n", a)
				usage(os.Stderr)
				return 1
			}
			input = a
		}
	}
	if input == "" {
		fmt.Fprintln(os.Stderr, "rotor minify: an input .luau/.lua file is required")
		usage(os.Stderr)
		return 1
	}

	src, err := os.ReadFile(input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "rotor minify: cannot read %q: %v\n", input, err)
		return 1
	}

	minified, diags := cst.Minify(string(src))
	if len(diags) != 0 {
		for _, d := range diags {
			fmt.Fprintf(os.Stderr, "rotor minify: %s:%d:%d: %s\n", input, d.Pos.Line, d.Pos.Col, d.Message)
		}
		return 1
	}

	if output == "" {
		_, _ = os.Stdout.WriteString(minified)
		return 0
	}
	if err := os.WriteFile(output, []byte(minified), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "rotor minify: cannot write %q: %v\n", output, err)
		return 1
	}
	return 0
}
