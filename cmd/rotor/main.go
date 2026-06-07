// Command rotor is the rotor CLI.
//
// Currently it provides `rotor check`, a fast native TypeScript project
// checker, and `rotor build`, a compile-to-Luau command that also emits the
// runtime library into the project's include folder and supports --type /
// --noInclude / --includePath (the rest of the rbxtsc build surface — watch,
// incremental, --luau, .d.ts emit — is later Phase 4 work).
package main

import (
	"fmt"
	"io"
	"os"
)

const banner = "rotor — native TypeScript-to-Luau compilation for roblox-ts projects"

// version is rotor's own release version, used for the `rotor build` emit
// header (`-- Compiled with rotor v...`). Library/test compilation keeps the
// upstream rbxtsc header so differential byte-comparison stays strict.
const version = "0.1.0-dev"

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
	case "build":
		return cmdBuild(args[1:])
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
	fmt.Fprintln(w, "  rotor check [path] [-w]   typecheck the project (native, full strictness)")
	fmt.Fprintln(w, "  rotor build [path]        compile the project to Luau (experimental — writes to tsconfig outDir")
	fmt.Fprintln(w, "                            and copies the runtime library to the include folder;")
	fmt.Fprintln(w, "                            no watch or incremental yet — Phase 4)")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Arguments:")
	fmt.Fprintln(w, "  path          project directory containing tsconfig.json (default \".\")")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Flags:")
	fmt.Fprintln(w, "  -w, --watch              re-run the check when watched files change (check only)")
	fmt.Fprintln(w, "  --type <kind>            override project type inference (build only; choices: game, model, package)")
	fmt.Fprintln(w, "  --noInclude              do not copy include files (build only)")
	fmt.Fprintln(w, "  -i, --includePath <dir>  folder to copy runtime files to (build only; default <path>/include)")
	fmt.Fprintln(w, "  -h, --help               show this help")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Exit codes: 0 = no errors, 1 = errors found, 2 = usage or config failure")
}
