package diagframe

import (
	"strings"
	"testing"
)

func TestRenderGroups_SummaryAndOrder(t *testing.T) {
	groups := []Group{
		{Path: "b.luau", Source: "y = 2\n", Lang: Luau, Spots: []Spot{{Offset: 0, Len: 1, Severity: Error, Message: "b err"}}},
		{Path: "a.luau", Source: "x = 1\n", Lang: Luau, Spots: []Spot{
			{Offset: 0, Len: 1, Severity: Error, Message: "a err"},
			{Offset: 0, Len: 1, Severity: Warning, Message: "a warn"},
		}},
	}
	out := RenderGroups(groups, Options{Color: false}, 0)
	if strings.Index(out, "a.luau") > strings.Index(out, "b.luau") {
		t.Error("files not sorted (a.luau should come before b.luau)")
	}
	if !strings.Contains(out, "2 errors") || !strings.Contains(out, "1 warning") {
		t.Errorf("summary counts wrong:\n%s", out)
	}
	if !strings.Contains(out, "in 2 files") {
		t.Errorf("file count wrong:\n%s", out)
	}
}

func TestRenderGroups_Truncation(t *testing.T) {
	var spots []Spot
	for i := 0; i < 5; i++ {
		spots = append(spots, Spot{Offset: 0, Len: 1, Severity: Error, Message: "e"})
	}
	groups := []Group{{Path: "a.luau", Source: "x = 1\n", Lang: Luau, Spots: spots}}
	out := RenderGroups(groups, Options{Color: false}, 2)
	if !strings.Contains(out, "and 3 more") {
		t.Errorf("expected truncation note:\n%s", out)
	}
}

func TestRenderGroups_NoColorIsASCII(t *testing.T) {
	groups := []Group{{Path: "a.luau", Source: "x = 1\n", Lang: Luau, Spots: []Spot{
		{Offset: 0, Len: 1, Severity: Error, Message: "e"},
		{Offset: 0, Len: 1, Severity: Warning, Message: "w"},
	}}}
	out := RenderGroups(groups, Options{Color: false}, 1) // forces a truncation note too
	for i := 0; i < len(out); i++ {
		if out[i] > 127 {
			t.Fatalf("non-ASCII byte %d at index %d in no-color output:\n%q", out[i], i, out)
		}
	}
}
