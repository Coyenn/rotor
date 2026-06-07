// Command rotor is the rotor CLI.
//
// `rotor check` is a fast native TypeScript project checker; `rotor build`
// compiles to Luau with the full rbxtsc build flag surface (ProjectOptions
// merged from defaults < tsconfig `rbxts` key < argv). Build watch mode is
// available; incremental rebuild selection remains later Phase 4 work.
//
// Exit-code policy: 0 = success, 1 = ANY failure including usage errors —
// matching upstream rbxtsc, whose yargs `.fail` handler sets exit code 1
// (CLI/cli.ts L30-35). rotor previously exited 2 for usage errors; that
// divergence was removed in Phase 4 for drop-in parity.
package main

import (
	"fmt"
	"io"
	"os"
)

const banner = "rotor — native TypeScript-to-Luau compilation for roblox-ts projects"

// version is rotor's own release version, used for `--version` and the
// `rotor build` emit header (`-- Compiled with rotor v...`). Library/test
// compilation keeps the upstream rbxtsc header so differential
// byte-comparison stays strict.
const version = "0.1.0-dev"

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		usage(os.Stderr)
		return 1
	}
	switch args[0] {
	case "check":
		return cmdCheck(args[1:])
	case "build":
		return cmdBuild(args[1:])
	case "help", "-h", "--help":
		usage(os.Stdout)
		return 0
	case "version", "-v", "--version":
		// Upstream prints the bare COMPILER_VERSION (CLI/cli.ts L14); rotor
		// prints its OWN version, not rbxtsc's.
		fmt.Println(version)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "rotor: unknown command %q\n\n", args[0])
		usage(os.Stderr)
		return 1
	}
}

func usage(w io.Writer) {
	fmt.Fprintln(w, banner)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  rotor check [path] [-w]      typecheck the project (native, full strictness)")
	fmt.Fprintln(w, "  rotor build [options] [path] compile the project to Luau (writes to tsconfig outDir")
	fmt.Fprintln(w, "                               and copies the runtime library to the include folder)")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Build options (rbxtsc-compatible; booleans accept --flag, --flag=false, --no-flag):")
	fmt.Fprintln(w, "  -p, --project <path>      project path (default \".\"): a tsconfig file, a directory")
	fmt.Fprintln(w, "                            containing one, or any path to search upward from")
	fmt.Fprintln(w, "  -w, --watch               enable watch mode")
	fmt.Fprintln(w, "  --usePolling              use polling for watch mode (requires --watch)")
	fmt.Fprintln(w, "  --verbose                 enable verbose logs")
	fmt.Fprintln(w, "  --noInclude               do not copy include files")
	fmt.Fprintln(w, "  --logTruthyChanges        logs changes to truthiness evaluation from Lua truthiness rules")
	fmt.Fprintln(w, "  --writeOnlyChanged        skip rewriting output files whose contents are unchanged")
	fmt.Fprintln(w, "  --writeTransformedFiles   not supported by rotor (parsed and ignored)")
	fmt.Fprintln(w, "  --optimizedLoops          numeric-for loop optimization (default true)")
	fmt.Fprintln(w, "  --type <kind>             override project type (choices: game, model, package)")
	fmt.Fprintln(w, "  -i, --includePath <dir>   folder to copy runtime files to (default <project>/include)")
	fmt.Fprintln(w, "  --rojo <path>             manually select Rojo project file")
	fmt.Fprintln(w, "  --allowCommentDirectives  allow @ts-ignore et al. (enforcement lands later in Phase 4)")
	fmt.Fprintln(w, "  --luau                    emit files with .luau extension (default true; --luau=false emits .lua)")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Options may also be set under the top-level \"rbxts\" key of tsconfig.json;")
	fmt.Fprintln(w, "merge order: defaults < rbxts < command line.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Other:")
	fmt.Fprintln(w, "  -h, --help                show this help")
	fmt.Fprintln(w, "  -v, --version             print rotor's version")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Exit codes: 0 = success, 1 = any failure (compile errors, config or usage errors — rbxtsc parity)")
}
