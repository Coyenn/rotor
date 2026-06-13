package bundle

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// isDataFile reports whether a resolved require path names a supported data file
// (rather than a Luau source module).
func isDataFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json", ".txt", ".md":
		return true
	}
	return false
}

// dataModuleSource builds the Luau source for a data-file module: a single
// `return <value>` whose value is the file's content rendered as a Luau literal.
//   - .json -> the parsed JSON as a Luau table/scalar literal.
//   - .txt/.md -> the file content as a Luau long-string literal.
//
// The returned source is plain Luau that cst.Parse accepts; the bundler parses
// it back so a data module flows through the exact same assembly/caching path as
// a code module (run-once memoization included).
func dataModuleSource(path string, content []byte) (string, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		return jsonModuleSource(path, content)
	default: // .txt / .md
		return "return " + luauLongString(string(content)) + "\n", nil
	}
}

// jsonModuleSource parses content as JSON and renders it as `return <luau>`.
func jsonModuleSource(path string, content []byte) (string, error) {
	dec := json.NewDecoder(bytes.NewReader(content))
	// UseNumber keeps integers exact and avoids 1.0-style float reformatting of
	// values that were written as integers.
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return "", fmt.Errorf("parsing JSON data file %s: %w", path, err)
	}
	if dec.More() {
		return "", fmt.Errorf("parsing JSON data file %s: trailing data after top-level value", path)
	}
	var b strings.Builder
	b.WriteString("return ")
	writeLuauValue(&b, v, 0)
	b.WriteString("\n")
	return b.String(), nil
}

// writeLuauValue renders a decoded JSON value as a Luau literal.
//
// Mapping rules:
//   - null    -> nil. A null array element becomes a `nil` hole (Luau arrays are
//     sparse, so this is a gap, matching Roblox JSONDecode); a null object value
//     drops the key entirely, because a Luau table literal `{ k = nil }` is the
//     same as omitting k. (Documented limitation: JSON nulls in objects are not
//     round-trippable.)
//   - bool    -> true/false
//   - number  -> the JSON number text verbatim (json.Number), which is valid Luau
//   - string  -> a double-quoted Luau short string with escapes
//   - array   -> a Luau array-style table { v1, v2, ... }
//   - object  -> a Luau table with identifier or ["key"] fields, keys sorted for
//     determinism
func writeLuauValue(b *strings.Builder, v any, depth int) {
	switch val := v.(type) {
	case nil:
		b.WriteString("nil")
	case bool:
		if val {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case json.Number:
		b.WriteString(string(val))
	case string:
		b.WriteString(luauShortString(val))
	case []any:
		writeLuauArray(b, val, depth)
	case map[string]any:
		writeLuauObject(b, val, depth)
	default:
		// json.Decode only produces the cases above; anything else is a bug.
		b.WriteString("nil")
	}
}

func writeLuauArray(b *strings.Builder, arr []any, depth int) {
	if len(arr) == 0 {
		b.WriteString("{}")
		return
	}
	b.WriteString("{\n")
	for _, item := range arr {
		b.WriteString(strings.Repeat("\t", depth+1))
		writeLuauValue(b, item, depth+1)
		b.WriteString(",\n")
	}
	b.WriteString(strings.Repeat("\t", depth))
	b.WriteString("}")
}

func writeLuauObject(b *strings.Builder, obj map[string]any, depth int) {
	keys := make([]string, 0, len(obj))
	for k := range obj {
		// A null object value maps to nil, which a table literal cannot carry,
		// so drop the key (documented above).
		if obj[k] == nil {
			continue
		}
		keys = append(keys, k)
	}
	if len(keys) == 0 {
		b.WriteString("{}")
		return
	}
	sort.Strings(keys)
	b.WriteString("{\n")
	for _, k := range keys {
		b.WriteString(strings.Repeat("\t", depth+1))
		b.WriteString(luauKey(k))
		b.WriteString(" = ")
		writeLuauValue(b, obj[k], depth+1)
		b.WriteString(",\n")
	}
	b.WriteString(strings.Repeat("\t", depth))
	b.WriteString("}")
}

// luauKey renders a JSON object key as a Luau table key: a bare identifier when
// it is a valid, non-keyword Luau identifier, otherwise a ["..."] computed key.
func luauKey(k string) string {
	if isLuauIdent(k) {
		return k
	}
	return "[" + luauShortString(k) + "]"
}

func isLuauIdent(s string) bool {
	if s == "" || luauReserved[s] {
		return false
	}
	for i, r := range s {
		isLetter := r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
		isDigit := r >= '0' && r <= '9'
		if i == 0 && !isLetter {
			return false
		}
		if i > 0 && !isLetter && !isDigit {
			return false
		}
	}
	return true
}

// luauReserved are Luau's reserved words, which cannot be bare table keys.
var luauReserved = map[string]bool{
	"and": true, "break": true, "do": true, "else": true, "elseif": true,
	"end": true, "false": true, "for": true, "function": true, "if": true,
	"in": true, "local": true, "nil": true, "not": true, "or": true,
	"repeat": true, "return": true, "then": true, "true": true,
	"until": true, "while": true,
}

// luauShortString renders s as a double-quoted Luau string literal with escapes.
// Control characters are emitted as \ddd decimal escapes (Luau-valid), printable
// characters pass through, and ", \, newline, etc. are backslash-escaped.
func luauShortString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '"':
			b.WriteString("\\\"")
		case '\\':
			b.WriteString("\\\\")
		case '\n':
			b.WriteString("\\n")
		case '\r':
			b.WriteString("\\r")
		case '\t':
			b.WriteString("\\t")
		default:
			if c < 0x20 {
				b.WriteString("\\")
				b.WriteString(strconv.Itoa(int(c)))
			} else {
				b.WriteByte(c)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// luauLongString renders s as a Luau long-string literal [[...]] / [==[...]==],
// choosing a bracket level whose closing delimiter does not occur in s (the same
// rule the Luau lexer enforces). It also guards a leading newline (Luau strips a
// first newline right after the opening bracket) by bumping past it, and prefixes
// the content with a newline so the first real line is preserved verbatim.
func luauLongString(s string) string {
	level := 0
	for {
		closeDelim := "]" + strings.Repeat("=", level) + "]"
		openDelim := "[" + strings.Repeat("=", level) + "["
		if !strings.Contains(s, closeDelim) && !strings.Contains(s, openDelim) {
			break
		}
		level++
	}
	eq := strings.Repeat("=", level)
	// Lead with a newline: Luau drops a single newline immediately following the
	// opening long bracket, so this keeps the file's true first byte intact.
	return "[" + eq + "[\n" + s + "]" + eq + "]"
}
