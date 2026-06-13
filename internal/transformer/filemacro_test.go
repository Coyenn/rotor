package transformer

import (
	"testing"

	"rotor/internal/luau/render"
)

// TestJSONBytesToLuau covers the JSON->Luau value builder: scalars, number
// precision (int vs float, no float64 round-trip), nested objects/arrays, null
// handling (object member dropped, array element -> nil), and key sorting.
func TestJSONBytesToLuau(t *testing.T) {
	cases := []struct {
		name string
		json string
		want string
	}{
		{"string", `"hi"`, `"hi"`},
		{"int", `42`, `42`},
		{"negative-int", `-7`, `-7`},
		{"float-precision", `0.30000000000000004`, `0.30000000000000004`},
		{"big-int-literal", `9007199254740993`, `9007199254740993`},
		{"bool-true", `true`, `true`},
		{"bool-false", `false`, `false`},
		{"null", `null`, `nil`},
		{"empty-object", `{}`, `{}`},
		{"empty-array", `[]`, `{}`},
		{"array", `[1, 2, 3]`, `{ 1, 2, 3 }`},
		{"array-with-null", `[1, null, 3]`, `{ 1, nil, 3 }`},
		// object keys are sorted; the null member is DROPPED entirely.
		{"object-sorted-null-dropped", `{"b":2,"a":1,"c":null}`, "{\n\ta = 1,\n\tb = 2,\n}"},
		// nested
		{"nested", `{"outer":{"inner":[true]}}`, "{\n\touter = {\n\t\tinner = { true },\n\t},\n}"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			expr, err := jsonBytesToLuau([]byte(tc.json))
			if err != nil {
				t.Fatalf("jsonBytesToLuau(%q): %v", tc.json, err)
			}
			got := render.Render(render.NewRenderState(), expr)
			if got != tc.want {
				t.Errorf("jsonBytesToLuau(%q):\ngot:  %q\nwant: %q", tc.json, got, tc.want)
			}
		})
	}
}

// TestJSONBytesToLuauInvalid: malformed JSON and trailing garbage both error.
func TestJSONBytesToLuauInvalid(t *testing.T) {
	for _, bad := range []string{`{`, `{ "x": }`, `not json`, `{} {}`, `1 2`} {
		if _, err := jsonBytesToLuau([]byte(bad)); err == nil {
			t.Errorf("jsonBytesToLuau(%q) = nil error, want a parse error", bad)
		}
	}
}
