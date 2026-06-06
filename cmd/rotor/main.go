// Command rotor is the rotor CLI.
//
// Currently it provides `rotor check`, a fast native TypeScript project
// checker (compilation to Luau is not yet implemented).
package main

import (
	"fmt"
	"io"
	"os"
)

const banner = "rotor check — native TypeScript checking (compilation to Luau not yet implemented)"

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		usage(os.Stderr)
		return 2
	}
	switch args[0] {
	case "check":
		return cmdCheck(args[1:])
	case "help", "-h", "--help":
		usage(os.Stdout)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "rotor: unknown command %q\n\n", args[0])
		usage(os.Stderr)
		return 2
	}
}

func usage(w io.Writer) {
	fmt.Fprintln(w, banner)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  rotor check [path] [-w]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Arguments:")
	fmt.Fprintln(w, "  path          project directory containing tsconfig.json (default \".\")")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Flags:")
	fmt.Fprintln(w, "  -w, --watch   re-run the check when watched files change")
	fmt.Fprintln(w, "  -h, --help    show this help")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Exit codes: 0 = no errors, 1 = errors found, 2 = usage or config failure")
}
