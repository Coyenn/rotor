package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"rotor/internal/compile"
	"rotor/internal/config"
)

// configTypeDeclarations is the `declare module "rotor/config"` declaration
// content written to rotor-config.d.ts so editors can type-check
// rotor.config.ts. Kept as a var (rather than using the constant inline) so
// the scaffold skips the file gracefully if the declarations are ever empty.
var configTypeDeclarations = config.TypeDeclarations

// cmdInit scaffolds a new rotor project. The game template (default) is a
// full rbxts game (package.json, tsconfig.json, DataModel Rojo project,
// starter src/); package is an rbxts model/package project; plain is a
// Luau-only project for bundle/minify/pack users.
func cmdInit(args []string) int {
	dir := ""
	template := "game"
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-h" || a == "--help":
			usage(os.Stdout)
			return 0
		case a == "-t" || a == "--template":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "rotor init: %s requires a template name (game|package|plain)\n", a)
				return 1
			}
			i++
			template = args[i]
		case strings.HasPrefix(a, "--template="):
			template = strings.TrimPrefix(a, "--template=")
		case strings.HasPrefix(a, "-t="):
			template = strings.TrimPrefix(a, "-t=")
		case strings.HasPrefix(a, "-"):
			fmt.Fprintf(os.Stderr, "rotor init: unknown flag %q\n\n", a)
			usage(os.Stderr)
			return 1
		default:
			if dir != "" {
				fmt.Fprintf(os.Stderr, "rotor init: unexpected extra argument %q\n\n", a)
				usage(os.Stderr)
				return 1
			}
			dir = a
		}
	}
	if dir == "" {
		dir = "."
	}
	if template != "game" && template != "package" && template != "plain" {
		fmt.Fprintf(os.Stderr, "rotor init: unknown template %q (want game, package, or plain)\n", template)
		return 1
	}

	abs, err := filepath.Abs(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "rotor init: %v\n", err)
		return 1
	}
	name := filepath.Base(abs)

	// Refuse to scaffold over an existing project.
	for _, marker := range []string{"package.json", "default.project.json"} {
		if fileExists(filepath.Join(dir, marker)) {
			fmt.Fprintf(os.Stderr,
				"rotor init: %s already exists in %s — refusing to scaffold into an existing project\n"+
					"(pick an empty or new directory, e.g. `rotor init my-game`)\n", marker, abs)
			return 1
		}
	}

	files := initFiles(template, name)
	if template != "plain" {
		if configTypeDeclarations != "" {
			files = append(files, initFile{config.TypeDeclarationsFileName, configTypeDeclarations})
		}
		// Editor types for the $env macro; the scaffolded tsconfig lists the
		// file under "include" so tsserver picks it up (the compiler skips its
		// own synthetic copy when this on-disk one is part of the program).
		files = append(files, initFile{compile.EnvDeclFileName, compile.EnvDeclFileText})
	}

	u := newUI(os.Stdout)
	u.banner("init  " + name)
	for _, f := range files {
		path := filepath.Join(dir, filepath.FromSlash(f.path))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			newUI(os.Stderr).failLine(fmt.Sprintf("rotor init: %v", err))
			return 1
		}
		if err := os.WriteFile(path, []byte(f.content), 0o644); err != nil {
			newUI(os.Stderr).failLine(fmt.Sprintf("rotor init: cannot write %q: %v", path, err))
			return 1
		}
		fmt.Printf("  %s %s\n", u.s.Green("+"), f.path)
	}

	printInitNextSteps(u, template, dir, len(files))
	return 0
}

// initFile is one scaffolded file: a forward-slash path relative to the
// project directory, and its content.
type initFile struct {
	path    string
	content string
}

// initFiles returns the scaffold for a template. name is the project
// directory's base name.
func initFiles(template, name string) []initFile {
	switch template {
	case "package":
		return []initFile{
			{"package.json", packageJSON(name, false)},
			{"tsconfig.json", tsconfigJSON(true)},
			{"default.project.json", fmt.Sprintf(`{
	"name": %s,
	"globIgnorePaths": ["**/package.json", "**/tsconfig.json"],
	"tree": {
		"$path": "out"
	}
}
`, jsonString(name))},
			{"src/init.ts", tsHelloModule},
			{".gitignore", "/node_modules\n/out\n"},
			{"rotor.config.ts", rotorConfigTS},
		}
	case "plain":
		return []initFile{
			{"default.project.json", fmt.Sprintf(`{
	"name": %s,
	"tree": {
		"$path": "src"
	}
}
`, jsonString(name))},
			{"src/init.luau", `--!strict
local hello = {}

function hello.makeHello(name: string): string
	return ("Hello from %s!"):format(name)
end

return hello
`},
			{"aftman.toml", `# Tool versions for this project, managed by Aftman:
#   https://github.com/LPGhatguy/aftman
#
# rotor bundle / minify / pack / sourcemap work without any of these; install
# rojo if you want live sync into Studio or rotor's rbxm/rbxmx pack formats.

[tools]
# rojo = "rojo-rbx/rojo@7.6.1"
`},
		}
	default: // game
		return []initFile{
			{"package.json", packageJSON(name, true)},
			{"tsconfig.json", tsconfigJSON(false)},
			{"default.project.json", fmt.Sprintf(`{
	"name": %s,
	"globIgnorePaths": ["**/package.json", "**/tsconfig.json"],
	"tree": {
		"$className": "DataModel",
		"ReplicatedStorage": {
			"rbxts_include": {
				"$path": "include",
				"node_modules": {
					"$className": "Folder",
					"@rbxts": {
						"$path": "node_modules/@rbxts"
					}
				}
			},
			"TS": {
				"$path": "out/shared"
			}
		},
		"ServerScriptService": {
			"TS": {
				"$path": "out/server"
			}
		},
		"StarterPlayer": {
			"StarterPlayerScripts": {
				"TS": {
					"$path": "out/client"
				}
			}
		},
		"HttpService": {
			"$properties": {
				"HttpEnabled": true
			}
		},
		"SoundService": {
			"$properties": {
				"RespectFilteringEnabled": true
			}
		}
	}
}
`, jsonString(name))},
			{"src/shared/module.ts", tsHelloModule},
			{"src/server/main.server.ts", `import { makeHello } from "../shared/module";

print(makeHello("main.server.ts"));
`},
			{"src/client/main.client.ts", `import { makeHello } from "../shared/module";

print(makeHello("main.client.ts"));
`},
			{".gitignore", "/node_modules\n/out\n/include/*\n!/include/.gitkeep\n"},
			{"include/.gitkeep", ""},
			{"rotor.config.ts", rotorConfigTS},
		}
	}
}

const tsHelloModule = `export function makeHello(name: string) {
	return ` + "`Hello from ${name}!`" + `;
}
`

const rotorConfigTS = `// rotor project configuration — read by ` + "`rotor asset sync`" + ` and
// ` + "`rotor deploy`" + `. Uncomment the sections you need.
import { defineConfig } from "rotor/config";

export default defineConfig({
	// assets: {
	// 	paths: ["assets/**/*.png", "assets/**/*.ogg"],
	// 	output: { luau: "src/shared/assets.luau", types: "src/shared/assets.d.ts" },
	// 	creator: { type: "user", id: 0 },
	// },
	// deploy: {
	// 	environments: {
	// 		dev: {
	// 			universeId: 0,
	// 			places: { start: { file: "build/game.rbxl", placeId: 0 } },
	// 		},
	// 	},
	// },
});
`

// packageJSON renders the scaffolded package.json. The name is normalized to
// a valid npm package name derived from the directory.
func packageJSON(name string, private bool) string {
	privateLine := ""
	if private {
		privateLine = "\t\"private\": true,\n"
	}
	return fmt.Sprintf(`{
	"name": %s,
	"version": "0.1.0",
%s	"scripts": {
		"build": "rotor build",
		"watch": "rotor dev"
	},
	"devDependencies": {
		"@rbxts/compiler-types": "^3.0.0",
		"@rbxts/types": "^1.0.800",
		"typescript": "^5.5.0"
	}
}
`, jsonString(npmName(name)), privateLine)
}

// tsconfigJSON renders the scaffolded tsconfig.json: the canonical rbxts shape
// (no baseUrl), with the jsx options present but commented out. tsconfig.json
// is JSONC, so the comment lines are valid for TypeScript tooling.
func tsconfigJSON(declaration bool) string {
	declarationLine := ""
	if declaration {
		declarationLine = "\t\t\"declaration\": true,\n"
	}
	return fmt.Sprintf(`{
	"compilerOptions": {
		// required
		"allowSyntheticDefaultImports": true,
		"module": "CommonJS",
		"moduleResolution": "Node",
		"noLib": true,
		"moduleDetection": "force",
		"strict": true,
		"target": "ESNext",
		"types": [],
		"typeRoots": ["node_modules/@rbxts"],

		// jsx — uncomment for @rbxts/react
		// "jsx": "react",
		// "jsxFactory": "React.createElement",
		// "jsxFragmentFactory": "React.Fragment",

		// configurable
%s		"rootDir": "src",
		"outDir": "out"
	},
	"include": ["src", "rotor-env.d.ts"]
}
`, declarationLine)
}

// npmName lowercases a directory name and replaces characters that are not
// valid in npm package names.
func npmName(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-._")
	if out == "" {
		return "rotor-project"
	}
	return out
}

// jsonString renders s as a JSON string literal.
func jsonString(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		return `"project"`
	}
	return string(b)
}

func printInitNextSteps(u *ui, template, dir string, n int) {
	fmt.Println()
	u.okLine(fmt.Sprintf("scaffolded a %s project", template), fmt.Sprintf("in %s · %s", dir, plural(n, "file")))
	fmt.Println()
	fmt.Printf("  %s\n", u.s.Bold("next steps"))
	step := func(cmd, why string) {
		if why == "" {
			fmt.Printf("    %s %s\n", u.s.Muted(u.s.Glyphs().Arrow), u.s.Info(cmd))
			return
		}
		pad := strings.Repeat(" ", max(1, 38-len(cmd)))
		fmt.Printf("    %s %s%s%s\n", u.s.Muted(u.s.Glyphs().Arrow), u.s.Info(cmd), pad, u.s.Muted(why))
	}
	if dir != "." {
		step("cd "+dir, "")
	}
	if template == "plain" {
		step("rotor pack --as luau -o bundle.luau", "package the project into one script")
		step("rotor sourcemap -o sourcemap.json", "generate a sourcemap for luau-lsp")
		return
	}
	step("npm install", "(or bun install / pnpm install)")
	step("rotor build", "compile to Luau")
	step("rotor dev", "watch + serve to Studio")
}
