package deploy

import (
	"context"
	"errors"
	"reflect"
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

// TestApplyGamePassWithSharedIcon: a badge and a game pass sharing one icon
// file upload it ONCE; both record the uploaded asset id in their outputs.
func TestApplyGamePassWithSharedIcon(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "shared.png", []byte("png"))

	price := int64(100)
	cfg := testConfig(config.Environment{
		UniverseID: 111,
		Badges:     map[string]config.Badge{"winner": {Name: "Winner!", Icon: "shared.png"}},
		GamePasses: map[string]config.GamePass{"vip": {Name: "VIP", Price: &price, Icon: "shared.png"}},
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
	if res.Created != 3 || res.Failed != 0 { // asset + badge + pass
		t.Fatalf("summary: %+v", res)
	}
	if len(fc.createdAssets) != 1 {
		t.Fatalf("shared icon uploaded %d times, want 1", len(fc.createdAssets))
	}
	if len(fc.createdGamePasses) != 1 || fc.createdGamePasses[0].Price == nil || *fc.createdGamePasses[0].Price != 100 {
		t.Fatalf("created passes: %+v", fc.createdGamePasses)
	}
	pass := st.Resources["game_pass/vip"]
	passID, _ := OutputInt64(pass.Outputs["gamePassId"])
	iconID, _ := OutputInt64(pass.Outputs["iconAssetId"])
	if passID != 301 || iconID != 701 {
		t.Fatalf("pass outputs = %v", pass.Outputs)
	}
}

// TestGamePassUpdateAndDelete: an update PATCHes the prior pass id with
// isForSale derived from price (nil = off sale); a delete takes the pass off
// sale.
func TestGamePassUpdateAndDelete(t *testing.T) {
	fc := &fakeCloud{}
	c := &Ctx{Cloud: fc, UniverseID: 111}
	prior := &StateEntry{InputsHash: "sha256:x", Outputs: map[string]any{"gamePassId": float64(33)}}

	// Price set -> for sale.
	price := int64(250)
	if _, err := (gamePassProvider{}).Update(context.Background(), c, GamePassInputs{Name: "VIP", Price: &price}, prior); err != nil {
		t.Fatal(err)
	}
	// Price nil -> off sale.
	if _, err := (gamePassProvider{}).Update(context.Background(), c, GamePassInputs{Name: "VIP"}, prior); err != nil {
		t.Fatal(err)
	}
	if len(fc.updatedGamePasses) != 2 {
		t.Fatalf("updates: %+v", fc.updatedGamePasses)
	}
	first, second := fc.updatedGamePasses[0], fc.updatedGamePasses[1]
	if first.ID != 33 || first.Req.Price == nil || *first.Req.Price != 250 || first.Req.IsForSale == nil || !*first.Req.IsForSale {
		t.Fatalf("priced update: %+v", first)
	}
	if second.Req.Price != nil || second.Req.IsForSale == nil || *second.Req.IsForSale {
		t.Fatalf("nil-price update: %+v", second)
	}

	// Delete -> off sale.
	if err := (gamePassProvider{}).Delete(context.Background(), c, prior); err != nil {
		t.Fatal(err)
	}
	last := fc.updatedGamePasses[len(fc.updatedGamePasses)-1]
	if last.ID != 33 || last.Req.IsForSale == nil || *last.Req.IsForSale {
		t.Fatalf("delete update: %+v", last)
	}
}

// TestThumbnailsFullReplace: creating uploads + orders the set; a reorder
// plans as an update that re-uploads, re-orders, and deletes the stale ids
// recorded by the previous apply (full-replace v1 semantics).
func TestThumbnailsFullReplace(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.png", []byte("aaa"))
	writeFile(t, dir, "b.png", []byte("bbb"))

	build := func(order []string) []Resource {
		cfg := testConfig(config.Environment{UniverseID: 111, Thumbnails: order})
		resources, _, err := BuildResources(dir, cfg, "dev")
		if err != nil {
			t.Fatal(err)
		}
		return resources
	}

	st := NewState()
	plan, err := BuildPlan(build([]string{"a.png", "b.png"}), st, PlanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	fc := &fakeCloud{}
	if _, err := Apply(context.Background(), plan, applyOpts(t, dir, fc, st)); err != nil {
		t.Fatal(err)
	}
	if len(fc.thumbnailUploads) != 2 || len(fc.thumbnailOrders) != 1 {
		t.Fatalf("create: uploads %v orders %v", fc.thumbnailUploads, fc.thumbnailOrders)
	}
	if !reflect.DeepEqual(fc.thumbnailOrders[0], []int64{901, 902}) {
		t.Fatalf("create order = %v", fc.thumbnailOrders[0])
	}
	if len(fc.deletedThumbnails) != 0 {
		t.Fatalf("create deleted %v", fc.deletedThumbnails)
	}

	// Reload state (ids round-trip through JSON as float64), reorder.
	st2, err := LoadState(StatePath(dir, "dev"))
	if err != nil {
		t.Fatal(err)
	}
	plan2, err := BuildPlan(build([]string{"b.png", "a.png"}), st2, PlanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if plan2.Updates != 1 {
		t.Fatalf("reorder plan: %+v", plan2)
	}
	if _, err := Apply(context.Background(), plan2, applyOpts(t, dir, fc, st2)); err != nil {
		t.Fatal(err)
	}
	if len(fc.thumbnailUploads) != 4 {
		t.Fatalf("update did not re-upload: %v", fc.thumbnailUploads)
	}
	if got := fc.thumbnailOrders[1]; !reflect.DeepEqual(got, []int64{903, 904}) {
		t.Fatalf("update order = %v", got)
	}
	// Stale ids from the first apply were deleted.
	if !reflect.DeepEqual(fc.deletedThumbnails, []int64{901, 902}) {
		t.Fatalf("stale deletes = %v", fc.deletedThumbnails)
	}
	ids, ok := OutputInt64Slice(st2.Resources["experience_thumbnails/thumbnails"].Outputs["thumbnailIds"])
	if !ok || !reflect.DeepEqual(ids, []int64{903, 904}) {
		t.Fatalf("recorded ids = %v (%v)", ids, ok)
	}

	// Removing the resource (with --allow-deletes) deletes the live set.
	plan3, err := BuildPlan(nil, st2, PlanOptions{AllowDeletes: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(context.Background(), plan3, applyOpts(t, dir, fc, st2)); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(fc.deletedThumbnails, []int64{901, 902, 903, 904}) {
		t.Fatalf("final deletes = %v", fc.deletedThumbnails)
	}
}

// TestDeveloperProductLifecycle: create records the product id, update
// PATCHes it, delete is state-only (no API call — Roblox cannot delete
// developer products).
func TestDeveloperProductLifecycle(t *testing.T) {
	fc := &fakeCloud{}
	c := &Ctx{Cloud: fc, UniverseID: 111}
	in := DeveloperProductInputs{Name: "Coins", Description: "100 coins", Price: 25}

	out, err := (developerProductProvider{}).Create(context.Background(), c, in, nil)
	if err != nil {
		t.Fatal(err)
	}
	if id, _ := OutputInt64(out["productId"]); id != 601 {
		t.Fatalf("outputs = %v", out)
	}
	if len(fc.createdProducts) != 1 || fc.createdProducts[0].PriceInRobux != 25 {
		t.Fatalf("creates: %+v", fc.createdProducts)
	}

	prior := &StateEntry{InputsHash: "sha256:x", Outputs: map[string]any{"productId": float64(601)}}
	in.Price = 50
	out, err = (developerProductProvider{}).Update(context.Background(), c, in, prior)
	if err != nil {
		t.Fatal(err)
	}
	if len(fc.updatedProducts) != 1 || fc.updatedProducts[0].ID != 601 || fc.updatedProducts[0].Req.PriceInRobux != 50 {
		t.Fatalf("updates: %+v", fc.updatedProducts)
	}
	if id, _ := OutputInt64(out["productId"]); id != 601 {
		t.Fatalf("update outputs = %v", out)
	}

	if err := (developerProductProvider{}).Delete(context.Background(), c, prior); err != nil {
		t.Fatal(err)
	}
	if len(fc.updatedProducts) != 1 || len(fc.createdProducts) != 1 {
		t.Fatal("delete made an API call; developer_product delete is state-only")
	}
}

// TestSocialLinkLifecycle: create maps the config type onto the API enum and
// records the id; update PATCHes the prior id; delete removes it.
func TestSocialLinkLifecycle(t *testing.T) {
	fc := &fakeCloud{}
	c := &Ctx{Cloud: fc, UniverseID: 111}
	in := SocialLinkInputs{Title: "Join us", URL: "https://discord.gg/x", Type: "discord"}

	out, err := (socialLinkProvider{}).Create(context.Background(), c, in, nil)
	if err != nil {
		t.Fatal(err)
	}
	if id, _ := OutputInt64(out["socialLinkId"]); id != 401 {
		t.Fatalf("outputs = %v", out)
	}
	if len(fc.createdLinks) != 1 || fc.createdLinks[0].Type != "Discord" {
		t.Fatalf("creates: %+v (type must map to the API enum)", fc.createdLinks)
	}

	prior := &StateEntry{InputsHash: "sha256:x", Outputs: map[string]any{"socialLinkId": float64(401)}}
	in.Title = "Join the server"
	if _, err := (socialLinkProvider{}).Update(context.Background(), c, in, prior); err != nil {
		t.Fatal(err)
	}
	if len(fc.updatedLinks) != 1 || fc.updatedLinks[0].ID != 401 || fc.updatedLinks[0].Req.Title != "Join the server" {
		t.Fatalf("updates: %+v", fc.updatedLinks)
	}

	if err := (socialLinkProvider{}).Delete(context.Background(), c, prior); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(fc.deletedLinks, []int64{401}) {
		t.Fatalf("deletes: %v", fc.deletedLinks)
	}

	// Unknown type errors before any API call.
	if _, err := (socialLinkProvider{}).Create(context.Background(), c, SocialLinkInputs{Type: "myspace"}, nil); err == nil {
		t.Fatal("invalid social link type not rejected")
	}
}

// TestPlaceConfigUpdateMask: only set fields enter the place PATCH mask,
// including maxPlayers (serverSize).
func TestPlaceConfigUpdateMask(t *testing.T) {
	cases := []struct {
		in       PlaceConfigInputs
		wantMask []string
	}{
		{PlaceConfigInputs{PlaceID: 1, Name: "Lobby"}, []string{"displayName"}},
		{PlaceConfigInputs{PlaceID: 1, Description: "d"}, []string{"description"}},
		{PlaceConfigInputs{PlaceID: 1, ServerSize: 30}, []string{"serverSize"}},
		{PlaceConfigInputs{PlaceID: 1, Name: "Lobby", Description: "d", ServerSize: 30},
			[]string{"displayName", "description", "serverSize"}},
	}
	for _, tc := range cases {
		fc := &fakeCloud{}
		c := &Ctx{Cloud: fc, UniverseID: 111}
		if _, err := (placeConfigProvider{}).Create(context.Background(), c, tc.in, nil); err != nil {
			t.Fatalf("%+v: %v", tc.in, err)
		}
		if len(fc.placeMasks) != 1 || !reflect.DeepEqual(fc.placeMasks[0], tc.wantMask) {
			t.Errorf("%+v: masks = %v, want %v", tc.in, fc.placeMasks, tc.wantMask)
		}
	}
	// Nothing set -> no API call.
	fc := &fakeCloud{}
	c := &Ctx{Cloud: fc, UniverseID: 111}
	if _, err := (placeConfigProvider{}).Create(context.Background(), c, PlaceConfigInputs{PlaceID: 1}, nil); err != nil {
		t.Fatal(err)
	}
	if len(fc.placeMasks) != 0 {
		t.Fatalf("empty inputs made an API call: %v", fc.placeMasks)
	}
}

// TestExperiencePrivateServerPriceMask: the privateServers price PATCHes
// privateServerPriceRobux (including an explicit 0 = free).
func TestExperiencePrivateServerPriceMask(t *testing.T) {
	for _, price := range []int64{0, 150} {
		fc := &fakeCloud{}
		c := &Ctx{Cloud: fc, UniverseID: 111}
		p := price
		if _, err := (experienceProvider{}).Create(context.Background(), c, ExperienceInputs{PrivateServerPrice: &p}, nil); err != nil {
			t.Fatal(err)
		}
		if len(fc.universeMasks) != 1 || !reflect.DeepEqual(fc.universeMasks[0], []string{"privateServerPriceRobux"}) {
			t.Fatalf("price %d: masks = %v", price, fc.universeMasks)
		}
		if fc.universePatches[0].PrivateServerPriceRobux != price {
			t.Fatalf("price %d: patch = %+v", price, fc.universePatches[0])
		}
	}
}

// TestIconUploadProvider: the experience icon uploads the file and records
// the returned image id.
func TestIconUploadProvider(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "icon.png", []byte("png"))
	fc := &fakeCloud{}
	c := &Ctx{Cloud: fc, UniverseID: 111, ProjectDir: dir}
	out, err := (experienceIconProvider{}).Create(context.Background(), c,
		ExperienceIconInputs{File: "icon.png", FileHash: "sha256:x"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if id, _ := OutputInt64(out["iconAssetId"]); id != 801 {
		t.Fatalf("outputs = %v", out)
	}
	if !reflect.DeepEqual(fc.iconUploads, []string{"icon.png"}) {
		t.Fatalf("uploads = %v", fc.iconUploads)
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
