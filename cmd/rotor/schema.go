package main

import (
	"fmt"
	"io"
	"os"

	"rotor/internal/config"
)

// cmdSchema prints the canonical rotor.toml JSON Schema (config.Schema) to
// stdout. Projects no longer carry a per-project rotor.schema.json: rotor.toml's
// `#:schema` directive points at the schema hosted on raw GitHub
// (config.SchemaDirective), so editors fetch it directly. This command emits the
// schema for two purposes — refreshing the committed file that the hosted URL
// serves, and giving a project that wants a local/offline copy an easy way to
// produce one:
//
//	rotor schema > rotor.schema.json
func cmdSchema(args []string) int {
	for _, a := range args {
		switch a {
		case "-h", "--help":
			fmt.Fprintln(os.Stdout, "rotor schema — print the rotor.toml JSON Schema to stdout")
			fmt.Fprintln(os.Stdout)
			fmt.Fprintln(os.Stdout, "Editors resolve the schema from the #:schema URL in rotor.toml; this")
			fmt.Fprintln(os.Stdout, "command emits the same schema for publishing or a local copy:")
			fmt.Fprintln(os.Stdout, "  rotor schema > rotor.schema.json")
			return 0
		default:
			fmt.Fprintf(os.Stderr, "rotor schema: unexpected argument %q\n", a)
			return 1
		}
	}
	return writeSchema(os.Stdout)
}

// writeSchema writes the JSON Schema to w verbatim. Split from cmdSchema so the
// emitted bytes can be asserted in tests without capturing os.Stdout.
func writeSchema(w io.Writer) int {
	if _, err := io.WriteString(w, config.Schema); err != nil {
		fmt.Fprintf(os.Stderr, "rotor schema: %v\n", err)
		return 1
	}
	return 0
}
