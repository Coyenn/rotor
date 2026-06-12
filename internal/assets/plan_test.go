package assets

import "testing"

func TestBuildPlan(t *testing.T) {
	scan := &ScanResult{
		Files: []File{
			{Path: "assets/changed.ogg", Type: TypeAudio, Hash: "sha256:new"},
			{Path: "assets/fresh.png", Type: TypeDecal, Hash: "sha256:f"},
			{Path: "assets/same.png", Type: TypeDecal, Hash: "sha256:s"},
		},
		Skipped: []string{"assets/readme.txt"},
	}
	lock := NewLockfile()
	lock.Assets["assets/changed.ogg"] = LockEntry{Hash: "sha256:old", AssetID: 50}
	lock.Assets["assets/same.png"] = LockEntry{Hash: "sha256:s", AssetID: 60}
	lock.Assets["assets/deleted.png"] = LockEntry{Hash: "sha256:d", AssetID: 70} // no longer on disk

	p := BuildPlan(scan, lock)

	if len(p.Items) != 3 {
		t.Fatalf("got %d items, want 3", len(p.Items))
	}
	byPath := map[string]PlanItem{}
	for _, it := range p.Items {
		byPath[it.File.Path] = it
	}
	if it := byPath["assets/fresh.png"]; it.Action != ActionCreate || it.AssetID != 0 {
		t.Fatalf("fresh: %+v", it)
	}
	if it := byPath["assets/changed.ogg"]; it.Action != ActionUpdate || it.AssetID != 50 {
		t.Fatalf("changed: %+v (update must keep the asset id)", it)
	}
	if it := byPath["assets/same.png"]; it.Action != ActionUnchanged || it.AssetID != 60 {
		t.Fatalf("same: %+v", it)
	}

	if p.Count(ActionCreate) != 1 || p.Count(ActionUpdate) != 1 || p.Count(ActionUnchanged) != 1 {
		t.Fatalf("counts: create=%d update=%d unchanged=%d", p.Count(ActionCreate), p.Count(ActionUpdate), p.Count(ActionUnchanged))
	}
	if p.Changes() != 2 {
		t.Fatalf("Changes() = %d, want 2", p.Changes())
	}
	if len(p.Skipped) != 1 || p.Skipped[0] != "assets/readme.txt" {
		t.Fatalf("skipped = %v", p.Skipped)
	}

	// Files deleted from disk keep their lock entries (no delete action in v1).
	if _, ok := lock.Assets["assets/deleted.png"]; !ok {
		t.Fatal("plan must not mutate the lockfile")
	}
}
