package cloud

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"
)

// Places v2 (cloud/v2) and place publishing v1. References:
// https://create.roblox.com/docs/cloud/reference/Place and
// https://create.roblox.com/docs/cloud/reference/patterns (publish is the
// older universes v1 surface, still the only Open Cloud place-file upload).
const (
	placePathFmt        = "/cloud/v2/universes/%d/places/%d"
	placePublishPathFmt = "/universes/v1/%d/places/%d/versions"
)

// Accepted versionType values for PublishPlaceVersion.
const (
	VersionTypeSaved     = "Saved"
	VersionTypePublished = "Published"
)

// Place mirrors the cloud/v2 Place resource.
type Place struct {
	Path        string `json:"path,omitempty"`
	CreateTime  string `json:"createTime,omitempty"`
	UpdateTime  string `json:"updateTime,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
	Description string `json:"description,omitempty"`
	ServerSize  int32  `json:"serverSize,omitempty"`
}

// GetPlace fetches GET /cloud/v2/universes/{universeId}/places/{placeId}.
func (c *Client) GetPlace(ctx context.Context, universeID, placeID int64) (Place, error) {
	var p Place
	err := c.do(ctx, "GET", fmt.Sprintf(placePathFmt, universeID, placeID), nil, "", nil, &p)
	return p, err
}

// UpdatePlace PATCHes the place with an updateMask query, like
// UpdateUniverse.
func (c *Client) UpdatePlace(ctx context.Context, universeID, placeID int64, p Place, updateMask []string) (Place, error) {
	q := url.Values{"updateMask": {strings.Join(updateMask, ",")}}
	var out Place
	err := c.doJSON(ctx, "PATCH", fmt.Sprintf(placePathFmt, universeID, placeID), q, p, &out)
	return out, err
}

// PublishPlaceVersion uploads a place file (rbxl/rbxlx bytes) via POST
// /universes/v1/{universeId}/places/{placeId}/versions?versionType=...,
// Content-Type application/octet-stream. versionType is "Saved" or
// "Published". Returns the new version number.
//
// The body is buffered in memory so retries can resend it; place files are
// typically a few MB.
func (c *Client) PublishPlaceVersion(ctx context.Context, universeID, placeID int64, versionType string, body io.Reader) (versionNumber int64, err error) {
	data, err := io.ReadAll(body)
	if err != nil {
		return 0, err
	}
	q := url.Values{"versionType": {versionType}}
	var out struct {
		VersionNumber int64 `json:"versionNumber"`
	}
	path := fmt.Sprintf(placePublishPathFmt, universeID, placeID)
	if err := c.do(ctx, "POST", path, q, "application/octet-stream", data, &out); err != nil {
		return 0, err
	}
	return out.VersionNumber, nil
}
