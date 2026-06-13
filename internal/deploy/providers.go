package deploy

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"

	"rotor/internal/cloud"
)

// ---------------------------------------------------------------- place_file

// PlaceFileInputs publishes a built place file to a place id. FileHash is
// the content hash of the file, so a rebuilt .rbxl plans as an update even
// though the path is unchanged.
type PlaceFileInputs struct {
	PlaceID     int64  `json:"placeId"`
	File        string `json:"file"`
	FileHash    string `json:"fileHash"`
	VersionType string `json:"versionType"` // "Saved" | "Published"
}

type placeFileProvider struct{}

func (placeFileProvider) Create(ctx context.Context, c *Ctx, inputs any, prior *StateEntry) (map[string]any, error) {
	return publishPlace(ctx, c, inputs)
}

func (placeFileProvider) Update(ctx context.Context, c *Ctx, inputs any, prior *StateEntry) (map[string]any, error) {
	return publishPlace(ctx, c, inputs)
}

// Delete is state-only: Open Cloud cannot unpublish a place version, so a
// removed place is simply forgotten.
func (placeFileProvider) Delete(ctx context.Context, c *Ctx, prior *StateEntry) error { return nil }

func publishPlace(ctx context.Context, c *Ctx, inputs any) (map[string]any, error) {
	in, err := decodeInputs[PlaceFileInputs](inputs)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(c.ResolvePath(in.File))
	if err != nil {
		return nil, fmt.Errorf("reading place file: %w", err)
	}
	version, err := c.Cloud.PublishPlaceVersion(ctx, c.UniverseID, in.PlaceID, in.VersionType, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return map[string]any{"placeId": in.PlaceID, "versionNumber": version}, nil
}

// -------------------------------------------------------------- place_config

// PlaceConfigInputs PATCHes a place's metadata. Only set (non-empty/non-zero)
// fields enter the updateMask.
type PlaceConfigInputs struct {
	PlaceID     int64  `json:"placeId"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	ServerSize  int32  `json:"serverSize,omitempty"`
}

type placeConfigProvider struct{}

func (placeConfigProvider) Create(ctx context.Context, c *Ctx, inputs any, prior *StateEntry) (map[string]any, error) {
	return patchPlace(ctx, c, inputs)
}

func (placeConfigProvider) Update(ctx context.Context, c *Ctx, inputs any, prior *StateEntry) (map[string]any, error) {
	return patchPlace(ctx, c, inputs)
}

// Delete is state-only: removing the resource stops managing the settings,
// it does not revert them.
func (placeConfigProvider) Delete(ctx context.Context, c *Ctx, prior *StateEntry) error { return nil }

func patchPlace(ctx context.Context, c *Ctx, inputs any) (map[string]any, error) {
	in, err := decodeInputs[PlaceConfigInputs](inputs)
	if err != nil {
		return nil, err
	}
	var p cloud.Place
	var mask []string
	if in.Name != "" {
		p.DisplayName = in.Name
		mask = append(mask, "displayName")
	}
	if in.Description != "" {
		p.Description = in.Description
		mask = append(mask, "description")
	}
	if in.ServerSize > 0 {
		p.ServerSize = in.ServerSize
		mask = append(mask, "serverSize")
	}
	outputs := map[string]any{"placeId": in.PlaceID}
	if len(mask) == 0 {
		return outputs, nil
	}
	if _, err := c.Cloud.UpdatePlace(ctx, c.UniverseID, in.PlaceID, p, mask); err != nil {
		return nil, err
	}
	return outputs, nil
}

// ---------------------------------------------------------------- experience

// ExperienceInputs PATCHes universe-level settings. Playability maps to the
// cloud/v2 visibility enum ("public" -> PUBLIC, "private"/"friends" ->
// PRIVATE; Open Cloud has no friends-only visibility). Payments is recorded
// for drift detection but has no cloud/v2 Universe field yet, so it does not
// reach the API in v1.
type ExperienceInputs struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Playability string `json:"playability,omitempty"` // "public" | "private" | "friends"
	Payments    string `json:"payments,omitempty"`
	// PrivateServerPrice maps to the v2 privateServerPriceRobux field; nil
	// leaves private-server pricing unmanaged.
	PrivateServerPrice *int64 `json:"privateServerPrice,omitempty"`
}

type experienceProvider struct{}

func (experienceProvider) Create(ctx context.Context, c *Ctx, inputs any, prior *StateEntry) (map[string]any, error) {
	return patchUniverse(ctx, c, inputs)
}

func (experienceProvider) Update(ctx context.Context, c *Ctx, inputs any, prior *StateEntry) (map[string]any, error) {
	return patchUniverse(ctx, c, inputs)
}

// Delete is state-only (settings are not reverted).
func (experienceProvider) Delete(ctx context.Context, c *Ctx, prior *StateEntry) error { return nil }

func patchUniverse(ctx context.Context, c *Ctx, inputs any) (map[string]any, error) {
	in, err := decodeInputs[ExperienceInputs](inputs)
	if err != nil {
		return nil, err
	}
	var u cloud.Universe
	var mask []string
	if in.Name != "" {
		u.DisplayName = in.Name
		mask = append(mask, "displayName")
	}
	if in.Description != "" {
		u.Description = in.Description
		mask = append(mask, "description")
	}
	if in.Playability != "" {
		switch in.Playability {
		case "public":
			u.Visibility = "PUBLIC"
		case "private", "friends":
			u.Visibility = "PRIVATE"
		default:
			return nil, fmt.Errorf("invalid playability %q", in.Playability)
		}
		mask = append(mask, "visibility")
	}
	if in.PrivateServerPrice != nil {
		u.PrivateServerPriceRobux = *in.PrivateServerPrice
		mask = append(mask, "privateServerPriceRobux")
	}
	outputs := map[string]any{"universeId": c.UniverseID}
	if len(mask) == 0 {
		return outputs, nil
	}
	if _, err := c.Cloud.UpdateUniverse(ctx, c.UniverseID, u, mask); err != nil {
		return nil, err
	}
	return outputs, nil
}

// --------------------------------------------------------------------- badge

// BadgeInputs creates/updates a badge. Icon, when set, is the NAME of the
// asset resource that uploads the icon file ("asset/<icon>"); the badge
// depends on it and records its uploaded asset id in outputs. (Open Cloud
// has no badge-icon update endpoint yet, so the association is informational
// in v1.)
type BadgeInputs struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Icon        string `json:"icon,omitempty"`
}

type badgeProvider struct{}

func (badgeProvider) Create(ctx context.Context, c *Ctx, inputs any, prior *StateEntry) (map[string]any, error) {
	in, err := decodeInputs[BadgeInputs](inputs)
	if err != nil {
		return nil, err
	}
	b, err := c.Cloud.CreateBadge(ctx, c.UniverseID, cloud.CreateBadgeRequest{
		Name:        in.Name,
		Description: in.Description,
	})
	if err != nil {
		return nil, err
	}
	outputs := map[string]any{"badgeId": b.ID}
	addBadgeIcon(c, in, outputs)
	return outputs, nil
}

func (badgeProvider) Update(ctx context.Context, c *Ctx, inputs any, prior *StateEntry) (map[string]any, error) {
	in, err := decodeInputs[BadgeInputs](inputs)
	if err != nil {
		return nil, err
	}
	badgeID, ok := priorInt64(prior, "badgeId")
	if !ok {
		return nil, fmt.Errorf("badge state has no badgeId; delete the state entry to recreate it")
	}
	if _, err := c.Cloud.UpdateBadge(ctx, badgeID, cloud.UpdateBadgeRequest{
		Name:        in.Name,
		Description: in.Description,
	}); err != nil {
		return nil, err
	}
	outputs := map[string]any{"badgeId": badgeID}
	addBadgeIcon(c, in, outputs)
	return outputs, nil
}

// Delete disables the badge (the legacy API has no hard delete) and forgets
// it from state.
func (badgeProvider) Delete(ctx context.Context, c *Ctx, prior *StateEntry) error {
	badgeID, ok := priorInt64(prior, "badgeId")
	if !ok {
		return nil // never created; nothing to disable
	}
	disabled := false
	_, err := c.Cloud.UpdateBadge(ctx, badgeID, cloud.UpdateBadgeRequest{Enabled: &disabled})
	return err
}

// addBadgeIcon copies the icon asset's uploaded id into the badge outputs
// when the badge declares an icon dependency.
func addBadgeIcon(c *Ctx, in BadgeInputs, outputs map[string]any) {
	if in.Icon == "" || c.Output == nil {
		return
	}
	if v, ok := c.Output(ResourceRef{Kind: KindAsset, Name: in.Icon}, "assetId"); ok {
		if id, ok := OutputInt64(v); ok {
			outputs["iconAssetId"] = id
		}
	}
}

func priorInt64(prior *StateEntry, key string) (int64, bool) {
	if prior == nil || prior.Outputs == nil {
		return 0, false
	}
	return OutputInt64(prior.Outputs[key])
}

// --------------------------------------------------------------------- asset

// AssetInputs uploads one file (badge icons in v1) via the Open Cloud assets
// API. CreatorType/CreatorID come from the config's assets.creator section
// and name who owns the uploaded asset.
type AssetInputs struct {
	File        string `json:"file"`
	FileHash    string `json:"fileHash"`
	AssetType   string `json:"assetType"` // "Decal" | "Audio"
	DisplayName string `json:"displayName"`
	CreatorType string `json:"creatorType,omitempty"` // "user" | "group"
	CreatorID   int64  `json:"creatorId,omitempty"`
}

type assetProvider struct{}

func (assetProvider) Create(ctx context.Context, c *Ctx, inputs any, prior *StateEntry) (map[string]any, error) {
	return uploadAsset(ctx, c, inputs)
}

// Update re-uploads as a NEW asset (new id): rotor deploy keeps its own
// minimal uploader and changed content gets a fresh asset, which dependents
// pick up through the output lookup.
func (assetProvider) Update(ctx context.Context, c *Ctx, inputs any, prior *StateEntry) (map[string]any, error) {
	return uploadAsset(ctx, c, inputs)
}

// Delete is state-only: Open Cloud has no asset delete; the asset is just no
// longer managed.
func (assetProvider) Delete(ctx context.Context, c *Ctx, prior *StateEntry) error { return nil }

func uploadAsset(ctx context.Context, c *Ctx, inputs any) (map[string]any, error) {
	in, err := decodeInputs[AssetInputs](inputs)
	if err != nil {
		return nil, err
	}
	var creator cloud.Creator
	switch in.CreatorType {
	case "user":
		creator.UserID = in.CreatorID
	case "group":
		creator.GroupID = in.CreatorID
	case "":
		return nil, fmt.Errorf("uploading %s needs an asset creator; set assets.creator { type, id } in rotor.config.ts", in.File)
	default:
		return nil, fmt.Errorf("invalid assets.creator.type %q (want \"user\" or \"group\")", in.CreatorType)
	}

	path := c.ResolvePath(in.File)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading asset file: %w", err)
	}
	opPath, err := c.Cloud.CreateAsset(ctx, cloud.CreateAssetRequest{
		AssetType:       in.AssetType,
		DisplayName:     in.DisplayName,
		CreationContext: cloud.CreationContext{Creator: creator},
	}, filepath.Base(path), bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	var asset cloud.Asset
	if err := c.Cloud.PollOperation(ctx, opPath, &asset); err != nil {
		return nil, err
	}
	return map[string]any{"assetId": asset.AssetID}, nil
}

// ----------------------------------------------------------------- game_pass

// GamePassInputs creates/updates a game pass. Price nil means the pass is
// not for sale. Icon, when set, names the asset resource that uploads the
// icon file ("asset/<icon>"), exactly like badges; the uploaded asset id is
// recorded in outputs (informational — Open Cloud has no pass-icon update
// endpoint yet).
type GamePassInputs struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Price       *int64 `json:"price,omitempty"`
	Icon        string `json:"icon,omitempty"`
}

type gamePassProvider struct{}

func (gamePassProvider) Create(ctx context.Context, c *Ctx, inputs any, prior *StateEntry) (map[string]any, error) {
	in, err := decodeInputs[GamePassInputs](inputs)
	if err != nil {
		return nil, err
	}
	g, err := c.Cloud.CreateGamePass(ctx, c.UniverseID, cloud.CreateGamePassRequest{
		Name:        in.Name,
		Description: in.Description,
		Price:       in.Price,
	})
	if err != nil {
		return nil, err
	}
	outputs := map[string]any{"gamePassId": g.GamePassID}
	addPassIcon(c, in, outputs)
	return outputs, nil
}

func (gamePassProvider) Update(ctx context.Context, c *Ctx, inputs any, prior *StateEntry) (map[string]any, error) {
	in, err := decodeInputs[GamePassInputs](inputs)
	if err != nil {
		return nil, err
	}
	passID, ok := priorInt64(prior, "gamePassId")
	if !ok {
		return nil, fmt.Errorf("game pass state has no gamePassId; delete the state entry to recreate it")
	}
	forSale := in.Price != nil
	if _, err := c.Cloud.UpdateGamePass(ctx, passID, cloud.UpdateGamePassRequest{
		Name:        in.Name,
		Description: in.Description,
		Price:       in.Price,
		IsForSale:   &forSale,
	}); err != nil {
		return nil, err
	}
	outputs := map[string]any{"gamePassId": passID}
	addPassIcon(c, in, outputs)
	return outputs, nil
}

// Delete takes the pass off sale (the legacy API has no hard delete) and
// forgets it from state.
func (gamePassProvider) Delete(ctx context.Context, c *Ctx, prior *StateEntry) error {
	passID, ok := priorInt64(prior, "gamePassId")
	if !ok {
		return nil // never created; nothing to take off sale
	}
	offSale := false
	_, err := c.Cloud.UpdateGamePass(ctx, passID, cloud.UpdateGamePassRequest{IsForSale: &offSale})
	return err
}

// addPassIcon copies the icon asset's uploaded id into the pass outputs when
// the pass declares an icon dependency.
func addPassIcon(c *Ctx, in GamePassInputs, outputs map[string]any) {
	if in.Icon == "" || c.Output == nil {
		return
	}
	if v, ok := c.Output(ResourceRef{Kind: KindAsset, Name: in.Icon}, "assetId"); ok {
		if id, ok := OutputInt64(v); ok {
			outputs["iconAssetId"] = id
		}
	}
}

// ----------------------------------------------------------- experience_icon

// ExperienceIconInputs uploads the experience's icon image. FileHash is the
// content hash so an edited image plans as an update.
type ExperienceIconInputs struct {
	File     string `json:"file"`
	FileHash string `json:"fileHash"`
}

type experienceIconProvider struct{}

func (experienceIconProvider) Create(ctx context.Context, c *Ctx, inputs any, prior *StateEntry) (map[string]any, error) {
	return uploadExperienceIcon(ctx, c, inputs)
}

func (experienceIconProvider) Update(ctx context.Context, c *Ctx, inputs any, prior *StateEntry) (map[string]any, error) {
	return uploadExperienceIcon(ctx, c, inputs)
}

// Delete is state-only: the icon cannot be removed via Open Cloud, it is
// just no longer managed.
func (experienceIconProvider) Delete(ctx context.Context, c *Ctx, prior *StateEntry) error {
	return nil
}

func uploadExperienceIcon(ctx context.Context, c *Ctx, inputs any) (map[string]any, error) {
	in, err := decodeInputs[ExperienceIconInputs](inputs)
	if err != nil {
		return nil, err
	}
	path := c.ResolvePath(in.File)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading icon file: %w", err)
	}
	id, err := c.Cloud.UploadUniverseIcon(ctx, c.UniverseID, filepath.Base(path), bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return map[string]any{"iconAssetId": id}, nil
}

// ----------------------------------------------------- experience_thumbnails

// ExperienceThumbnailsInputs manages the universe's ordered thumbnail set as
// ONE resource: Files is the configured order and FileHashes the matching
// content hashes, so adding, removing, editing, or reordering any thumbnail
// changes the inputs hash and plans as a single update.
//
// v1 simplification (deliberate): any change is a full replace — every file
// is re-uploaded, the new order is applied, and every thumbnail id recorded
// by the previous apply is deleted. Per-thumbnail ids are kept in outputs
// ("thumbnailIds", in display order) so the stale set is always known;
// a smarter diff (re-upload only changed files) can slot in later without a
// state migration.
type ExperienceThumbnailsInputs struct {
	Files      []string `json:"files"`
	FileHashes []string `json:"fileHashes"`
}

type experienceThumbnailsProvider struct{}

func (experienceThumbnailsProvider) Create(ctx context.Context, c *Ctx, inputs any, prior *StateEntry) (map[string]any, error) {
	return replaceThumbnails(ctx, c, inputs, nil)
}

func (experienceThumbnailsProvider) Update(ctx context.Context, c *Ctx, inputs any, prior *StateEntry) (map[string]any, error) {
	stale, _ := priorThumbnailIDs(prior)
	return replaceThumbnails(ctx, c, inputs, stale)
}

// Delete removes every thumbnail this resource created (ids from the prior
// outputs).
func (experienceThumbnailsProvider) Delete(ctx context.Context, c *Ctx, prior *StateEntry) error {
	ids, _ := priorThumbnailIDs(prior)
	for _, id := range ids {
		if err := c.Cloud.DeleteUniverseThumbnail(ctx, c.UniverseID, id); err != nil {
			return err
		}
	}
	return nil
}

// replaceThumbnails uploads the configured set in order, applies the order,
// then deletes the stale ids a previous apply created.
func replaceThumbnails(ctx context.Context, c *Ctx, inputs any, stale []int64) (map[string]any, error) {
	in, err := decodeInputs[ExperienceThumbnailsInputs](inputs)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(in.Files))
	for _, file := range in.Files {
		path := c.ResolvePath(file)
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading thumbnail file: %w", err)
		}
		id, err := c.Cloud.UploadUniverseThumbnail(ctx, c.UniverseID, filepath.Base(path), bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("uploading thumbnail %s: %w", file, err)
		}
		ids = append(ids, id)
	}
	if len(ids) > 0 {
		if err := c.Cloud.SetUniverseThumbnailOrder(ctx, c.UniverseID, ids); err != nil {
			return nil, fmt.Errorf("ordering thumbnails: %w", err)
		}
	}
	for _, id := range stale {
		if err := c.Cloud.DeleteUniverseThumbnail(ctx, c.UniverseID, id); err != nil {
			return nil, fmt.Errorf("deleting stale thumbnail %d: %w", id, err)
		}
	}
	return map[string]any{"thumbnailIds": ids}, nil
}

func priorThumbnailIDs(prior *StateEntry) ([]int64, bool) {
	if prior == nil || prior.Outputs == nil {
		return nil, false
	}
	return OutputInt64Slice(prior.Outputs["thumbnailIds"])
}

// --------------------------------------------------------- developer_product

// DeveloperProductInputs creates/updates a developer product.
type DeveloperProductInputs struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Price       int64  `json:"price"`
}

type developerProductProvider struct{}

func (developerProductProvider) Create(ctx context.Context, c *Ctx, inputs any, prior *StateEntry) (map[string]any, error) {
	in, err := decodeInputs[DeveloperProductInputs](inputs)
	if err != nil {
		return nil, err
	}
	p, err := c.Cloud.CreateDeveloperProduct(ctx, c.UniverseID, cloud.CreateDeveloperProductRequest{
		Name:         in.Name,
		Description:  in.Description,
		PriceInRobux: in.Price,
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"productId": p.ID}, nil
}

func (developerProductProvider) Update(ctx context.Context, c *Ctx, inputs any, prior *StateEntry) (map[string]any, error) {
	in, err := decodeInputs[DeveloperProductInputs](inputs)
	if err != nil {
		return nil, err
	}
	productID, ok := priorInt64(prior, "productId")
	if !ok {
		return nil, fmt.Errorf("developer product state has no productId; delete the state entry to recreate it")
	}
	if _, err := c.Cloud.UpdateDeveloperProduct(ctx, c.UniverseID, productID, cloud.UpdateDeveloperProductRequest{
		Name:         in.Name,
		Description:  in.Description,
		PriceInRobux: in.Price,
	}); err != nil {
		return nil, err
	}
	return map[string]any{"productId": productID}, nil
}

// Delete is state-only: Roblox has no developer-product delete; the product
// is just no longer managed.
func (developerProductProvider) Delete(ctx context.Context, c *Ctx, prior *StateEntry) error {
	return nil
}

// --------------------------------------------------------------- social_link

// SocialLinkInputs creates/updates a universe social link. Type is the
// config-level lowercase enum; the provider maps it to the API spelling.
type SocialLinkInputs struct {
	Title string `json:"title"`
	URL   string `json:"url"`
	Type  string `json:"type"` // facebook|twitter|youtube|twitch|discord|github|guilded
}

// socialLinkAPITypes maps the config enum onto the legacy API's PascalCase
// type names.
var socialLinkAPITypes = map[string]string{
	"facebook": "Facebook",
	"twitter":  "Twitter",
	"youtube":  "YouTube",
	"twitch":   "Twitch",
	"discord":  "Discord",
	"github":   "GitHub",
	"guilded":  "Guilded",
}

type socialLinkProvider struct{}

func (socialLinkProvider) Create(ctx context.Context, c *Ctx, inputs any, prior *StateEntry) (map[string]any, error) {
	req, err := socialLinkRequest(inputs)
	if err != nil {
		return nil, err
	}
	s, err := c.Cloud.CreateSocialLink(ctx, c.UniverseID, req)
	if err != nil {
		return nil, err
	}
	return map[string]any{"socialLinkId": s.ID}, nil
}

func (socialLinkProvider) Update(ctx context.Context, c *Ctx, inputs any, prior *StateEntry) (map[string]any, error) {
	req, err := socialLinkRequest(inputs)
	if err != nil {
		return nil, err
	}
	linkID, ok := priorInt64(prior, "socialLinkId")
	if !ok {
		return nil, fmt.Errorf("social link state has no socialLinkId; delete the state entry to recreate it")
	}
	if _, err := c.Cloud.UpdateSocialLink(ctx, c.UniverseID, linkID, req); err != nil {
		return nil, err
	}
	return map[string]any{"socialLinkId": linkID}, nil
}

// Delete removes the link from the universe.
func (socialLinkProvider) Delete(ctx context.Context, c *Ctx, prior *StateEntry) error {
	linkID, ok := priorInt64(prior, "socialLinkId")
	if !ok {
		return nil // never created; nothing to remove
	}
	return c.Cloud.DeleteSocialLink(ctx, c.UniverseID, linkID)
}

func socialLinkRequest(inputs any) (cloud.SocialLinkRequest, error) {
	in, err := decodeInputs[SocialLinkInputs](inputs)
	if err != nil {
		return cloud.SocialLinkRequest{}, err
	}
	apiType, ok := socialLinkAPITypes[in.Type]
	if !ok {
		return cloud.SocialLinkRequest{}, fmt.Errorf("invalid social link type %q", in.Type)
	}
	return cloud.SocialLinkRequest{Title: in.Title, URL: in.URL, Type: apiType}, nil
}
