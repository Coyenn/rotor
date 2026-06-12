package deploy

import (
	"context"
	"errors"
	"testing"

	"rotor/internal/config"
)

// applyOpts bundles the common Apply fixtures: a temp project, a fake cloud,
// and a state file under the project's .rotor dir.
func applyOpts(t *testing.T, dir string, fc *fakeCloud, st *State) ApplyOptions {
	t.Helper()
	path := StatePath(dir, "dev")
	return ApplyOptions{
		Providers:  DefaultProviders(),
		Cloud:      fc,
		UniverseID: 111,
		ProjectDir: dir,
		State:      st,
		SaveState:  func(s *State) error { return s.Save(path) },
	}
}

// TestApplyBadgeWithIconAsset runs the full asset -> badge chain: the asset
// uploads (create + poll), the badge creates, and the badge outputs carry
// the uploaded asset id read through Ctx.Output.
func TestApplyBadgeWithIconAsset(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "game.rbxl", []byte("rbxl"))
	writeFile(t, dir, "icon.png", []byte("png"))

	cfg := testConfig(config.Environment{
		UniverseID: 111,
		Places:     map[string]config.PlaceDeploy{"start": {File: "game.rbxl", PlaceID: 222}},
		Badges:     map[string]config.Badge{"winner": {Name: "Winner!", Icon: "icon.png"}},
	})
	resources, _, err := BuildResources(dir, cfg, "dev")
	if err != nil {
		t.Fatal(err)
	}
	st := NewState()
	plan, err := BuildPlan(resources, st, PlanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	fc := &fakeCloud{}
	res, err := Apply(context.Background(), plan, applyOpts(t, dir, fc, st))
	if err != nil {
		t.Fatal(err)
	}
	if res.Created != 3 || res.Failed != 0 {
		t.Fatalf("summary: %+v", res)
	}
	if len(fc.publishes) != 1 || fc.publishes[0].PlaceID != 222 || fc.publishes[0].VersionType != "Published" {
		t.Fatalf("publishes: %+v", fc.publishes)
	}
	if len(fc.createdAssets) != 1 || fc.createdAssets[0].CreationContext.Creator.GroupID != 99 {
		t.Fatalf("assets: %+v", fc.createdAssets)
	}
	badge := st.Resources["badge/winner"]
	if badge == nil {
		t.Fatal("badge missing from state")
	}
	badgeID, _ := OutputInt64(badge.Outputs["badgeId"])
	iconID, _ := OutputInt64(badge.Outputs["iconAssetId"])
	if badgeID != 501 || iconID != 701 {
		t.Fatalf("badge outputs = %v", badge.Outputs)
	}
	// State was persisted after each resource.
	loaded, err := LoadState(StatePath(dir, "dev"))
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Resources) != 3 {
		t.Fatalf("persisted state has %d resources", len(loaded.Resources))
	}
}

// TestApplyResume fails the second of two independent places, confirms the
// first persisted, then re-plans and applies only the remainder.
func TestApplyResume(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "one.rbxl", []byte("one"))
	writeFile(t, dir, "two.rbxl", []byte("two"))
	resources := []Resource{
		{Kind: KindPlaceFile, Name: "one", Inputs: PlaceFileInputs{PlaceID: 1, File: "one.rbxl", FileHash: "sha256:1", VersionType: "Published"}},
		{Kind: KindPlaceFile, Name: "two", Inputs: PlaceFileInputs{PlaceID: 2, File: "two.rbxl", FileHash: "sha256:2", VersionType: "Published"}},
	}

	st := NewState()
	plan, err := BuildPlan(resources, st, PlanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	fc := &fakeCloud{failPublish: map[int64]error{2: errors.New("boom")}}
	res, err := Apply(context.Background(), plan, applyOpts(t, dir, fc, st))
	if err != nil {
		t.Fatal(err)
	}
	if res.Created != 1 || res.Failed != 1 {
		t.Fatalf("first run: %+v", res)
	}

	// Resume from the persisted state: only "two" is left to create.
	st2, err := LoadState(StatePath(dir, "dev"))
	if err != nil {
		t.Fatal(err)
	}
	plan2, err := BuildPlan(resources, st2, PlanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if plan2.Creates != 1 || plan2.Noops != 1 {
		t.Fatalf("resume plan: %+v", plan2)
	}
	fc.failPublish = nil
	res2, err := Apply(context.Background(), plan2, applyOpts(t, dir, fc, st2))
	if err != nil {
		t.Fatal(err)
	}
	if res2.Created != 1 || res2.Unchanged != 1 || res2.Failed != 0 {
		t.Fatalf("second run: %+v", res2)
	}
	// "one" was published exactly once across both runs.
	count := 0
	for _, p := range fc.publishes {
		if p.PlaceID == 1 {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("place 1 published %d times", count)
	}
}

// TestApplyDependentSkipped fails the icon asset and expects the dependent
// badge to be skipped while the independent place still applies.
func TestApplyDependentSkipped(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "game.rbxl", []byte("rbxl"))
	writeFile(t, dir, "icon.png", []byte("png"))

	cfg := testConfig(config.Environment{
		UniverseID: 111,
		Places:     map[string]config.PlaceDeploy{"start": {File: "game.rbxl", PlaceID: 222}},
		Badges:     map[string]config.Badge{"winner": {Name: "Winner!", Icon: "icon.png"}},
	})
	resources, _, err := BuildResources(dir, cfg, "dev")
	if err != nil {
		t.Fatal(err)
	}
	st := NewState()
	plan, err := BuildPlan(resources, st, PlanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	fc := &fakeCloud{failCreateAsset: errors.New("moderation")}
	res, err := Apply(context.Background(), plan, applyOpts(t, dir, fc, st))
	if err != nil {
		t.Fatal(err)
	}
	if res.Created != 1 || res.Failed != 1 || res.Skipped != 1 {
		t.Fatalf("summary: %+v", res)
	}
	if len(fc.createdBadges) != 0 {
		t.Fatalf("badge created despite failed icon: %+v", fc.createdBadges)
	}
	if len(fc.publishes) != 1 {
		t.Fatalf("independent place not applied: %+v", fc.publishes)
	}
	statuses := map[string]StepStatus{}
	for _, r := range res.Results {
		statuses[r.Step.Ref.Key()] = r.Status
	}
	if statuses["asset/icon.png"] != StatusFailed || statuses["badge/winner"] != StatusSkipped || statuses["place_file/start"] != StatusApplied {
		t.Fatalf("statuses: %v", statuses)
	}
}

// TestApplyBlockedDeleteErrors: Apply refuses a plan with blocked deletes.
func TestApplyBlockedDeleteErrors(t *testing.T) {
	st := NewState()
	st.Resources["badge/old"] = &StateEntry{InputsHash: "sha256:x", Outputs: map[string]any{"badgeId": float64(900)}}
	plan, err := BuildPlan(nil, st, PlanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(context.Background(), plan, applyOpts(t, t.TempDir(), &fakeCloud{}, st)); err == nil {
		t.Fatal("blocked delete did not error apply")
	}
}

// TestApplyDelete: with --allow-deletes a removed badge is disabled and
// dropped from state.
func TestApplyDelete(t *testing.T) {
	dir := t.TempDir()
	st := NewState()
	st.Resources["badge/old"] = &StateEntry{InputsHash: "sha256:x", Outputs: map[string]any{"badgeId": float64(900)}}
	plan, err := BuildPlan(nil, st, PlanOptions{AllowDeletes: true})
	if err != nil {
		t.Fatal(err)
	}
	fc := &fakeCloud{}
	res, err := Apply(context.Background(), plan, applyOpts(t, dir, fc, st))
	if err != nil {
		t.Fatal(err)
	}
	if res.Deleted != 1 {
		t.Fatalf("summary: %+v", res)
	}
	if len(fc.updatedBadges) != 1 || fc.updatedBadges[0].ID != 900 ||
		fc.updatedBadges[0].Req.Enabled == nil || *fc.updatedBadges[0].Req.Enabled {
		t.Fatalf("badge not disabled: %+v", fc.updatedBadges)
	}
	if _, ok := st.Resources["badge/old"]; ok {
		t.Fatal("deleted badge still in state")
	}
	loaded, err := LoadState(StatePath(dir, "dev"))
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Resources) != 0 {
		t.Fatalf("persisted state still has %d resources", len(loaded.Resources))
	}
}

// TestExperienceUpdateMask: only the fields the config sets enter the
// updateMask, and playability maps onto the v2 visibility enum.
func TestExperienceUpdateMask(t *testing.T) {
	cases := []struct {
		in       ExperienceInputs
		wantMask []string
		wantVis  string
	}{
		{ExperienceInputs{Name: "X"}, []string{"displayName"}, ""},
		{ExperienceInputs{Description: "d"}, []string{"description"}, ""},
		{ExperienceInputs{Name: "X", Playability: "public"}, []string{"displayName", "visibility"}, "PUBLIC"},
		{ExperienceInputs{Playability: "private"}, []string{"visibility"}, "PRIVATE"},
		{ExperienceInputs{Playability: "friends"}, []string{"visibility"}, "PRIVATE"},
	}
	for _, tc := range cases {
		fc := &fakeCloud{}
		c := &Ctx{Cloud: fc, UniverseID: 111}
		if _, err := (experienceProvider{}).Create(context.Background(), c, tc.in, nil); err != nil {
			t.Fatalf("%+v: %v", tc.in, err)
		}
		if len(fc.universeMasks) != 1 {
			t.Fatalf("%+v: %d API calls", tc.in, len(fc.universeMasks))
		}
		got := fc.universeMasks[0]
		if len(got) != len(tc.wantMask) {
			t.Fatalf("%+v: mask = %v, want %v", tc.in, got, tc.wantMask)
		}
		for i := range got {
			if got[i] != tc.wantMask[i] {
				t.Fatalf("%+v: mask = %v, want %v", tc.in, got, tc.wantMask)
			}
		}
		if tc.wantVis != "" && fc.universePatches[0].Visibility != tc.wantVis {
			t.Fatalf("%+v: visibility = %q, want %q", tc.in, fc.universePatches[0].Visibility, tc.wantVis)
		}
	}

	// Payments alone is drift-tracked but produces no API call (no v2 field).
	fc := &fakeCloud{}
	c := &Ctx{Cloud: fc, UniverseID: 111}
	if _, err := (experienceProvider{}).Create(context.Background(), c, ExperienceInputs{Payments: "free"}, nil); err != nil {
		t.Fatal(err)
	}
	if len(fc.universeMasks) != 0 {
		t.Fatalf("payments-only made an API call: %v", fc.universeMasks)
	}
}

// TestBadgeUpdateUsesPriorID: an update PATCHes the badge id recorded at
// create time (surviving the float64 round-trip through JSON state).
func TestBadgeUpdateUsesPriorID(t *testing.T) {
	fc := &fakeCloud{}
	c := &Ctx{Cloud: fc, UniverseID: 111}
	prior := &StateEntry{InputsHash: "sha256:x", Outputs: map[string]any{"badgeId": float64(555)}}
	out, err := (badgeProvider{}).Update(context.Background(), c, BadgeInputs{Name: "N2", Description: "d2"}, prior)
	if err != nil {
		t.Fatal(err)
	}
	if len(fc.updatedBadges) != 1 || fc.updatedBadges[0].ID != 555 || fc.updatedBadges[0].Req.Name != "N2" {
		t.Fatalf("update calls: %+v", fc.updatedBadges)
	}
	if id, _ := OutputInt64(out["badgeId"]); id != 555 {
		t.Fatalf("outputs = %v", out)
	}
}
