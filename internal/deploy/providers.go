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
