package main

import "testing"

func TestCmdPackArgValidation(t *testing.T) {
	// unknown format -> exit 1
	if code := cmdPack([]string{".", "--as", "json"}); code != 1 {
		t.Fatalf("unknown format: exit %d", code)
	}
	// rbxm/rbxmx require -o
	if code := cmdPack([]string{".", "--as", "rbxm"}); code != 1 {
		t.Fatalf("rbxm without -o: exit %d", code)
	}
	// --entry only valid for luau
	if code := cmdPack([]string{".", "--as", "rbxmx", "-o", "x.rbxmx", "--entry", "a.b"}); code != 1 {
		t.Fatalf("entry on rbxmx: exit %d", code)
	}
	// unknown flag
	if code := cmdPack([]string{"--bogus"}); code != 1 {
		t.Fatalf("unknown flag: exit %d", code)
	}
}
