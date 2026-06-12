package main

import (
	"fmt"
	"os"
	"strings"

	"rotor/internal/bundle"
	"rotor/internal/luau/cst"
)

// cmdBundle bundles a Luau require graph rooted at an entry file into one runnable
// file. Output goes to --output, or stdout when no output path is given. With
// --minify the bundle is also minified.
func cmdBundle(args []string) int {
	entry := ""
	output := ""
	minify := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-h" || a == "--help":
			usage(os.Stdout)
			return 0
		case a == "--minify":
			minify = true
		case a == "-o" || a == "--output":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "rotor bundle: %s requires a path\n", a)
				return 1
			}
			i++
			output = args[i]
		case strings.HasPrefix(a, "--output="):
			output = strings.TrimPrefix(a, "--output=")
		case strings.HasPrefix(a, "-o="):
			output = strings.TrimPrefix(a, "-o=")
		case strings.HasPrefix(a, "-"):
			fmt.Fprintf(os.Stderr, "rotor bundle: unknown flag %q\n\n", a)
			usage(os.Stderr)
			return 1
		default:
			if entry != "" {
				fmt.Fprintf(os.Stderr, "rotor bundle: unexpected extra argument %q\n\n", a)
				usage(os.Stderr)
				return 1
			}
			entry = a
		}
	}
	if entry == "" {
		fmt.Fprintln(os.Stderr, "rotor bundle: an entry .luau/.lua file is required")
		usage(os.Stderr)
		return 1
	}

	out, err := bundle.Bundle(entry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "rotor bundle: %v\n", err)
		return 1
	}

	if minify {
		minified, diags := cst.Minify(out)
		if len(diags) != 0 {
			fmt.Fprintf(os.Stderr, "rotor bundle: internal error minifying bundle: %s\n", diags[0].Message)
			return 1
		}
		out = minified
	}

	if output == "" {
		_, _ = os.Stdout.WriteString(out)
		return 0
	}
	if err := os.WriteFile(output, []byte(out), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "rotor bundle: cannot write %q: %v\n", output, err)
		return 1
	}
	return 0
}
