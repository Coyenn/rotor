package cloud

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

// Universes v2 (cloud/v2). Reference:
// https://create.roblox.com/docs/cloud/reference/Universe
const universePathFmt = "/cloud/v2/universes/%d"

// Universe mirrors the cloud/v2 Universe resource (the subset deploy
// manages). Read-only fields (path, timestamps, owner) decode on GET and are
// ignored by PATCH thanks to updateMask.
type Universe struct {
	Path        string `json:"path,omitempty"`
	CreateTime  string `json:"createTime,omitempty"`
	UpdateTime  string `json:"updateTime,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
	Description string `json:"description,omitempty"`
	User        string `json:"user,omitempty"`
	Group       string `json:"group,omitempty"`
	// Visibility: "VISIBILITY_UNSPECIFIED" | "PUBLIC" | "PRIVATE".
	Visibility              string `json:"visibility,omitempty"`
	VoiceChatEnabled        bool   `json:"voiceChatEnabled,omitempty"`
	AgeRating               string `json:"ageRating,omitempty"`
	PrivateServerPriceRobux int64  `json:"privateServerPriceRobux,omitempty"`
	DesktopEnabled          bool   `json:"desktopEnabled,omitempty"`
	MobileEnabled           bool   `json:"mobileEnabled,omitempty"`
	TabletEnabled           bool   `json:"tabletEnabled,omitempty"`
	ConsoleEnabled          bool   `json:"consoleEnabled,omitempty"`
	VREnabled               bool   `json:"vrEnabled,omitempty"`
}

// GetUniverse fetches GET /cloud/v2/universes/{universeId}.
func (c *Client) GetUniverse(ctx context.Context, universeID int64) (Universe, error) {
	var u Universe
	err := c.do(ctx, "GET", fmt.Sprintf(universePathFmt, universeID), nil, "", nil, &u)
	return u, err
}

// UpdateUniverse PATCHes /cloud/v2/universes/{universeId} with an updateMask
// query naming the fields to change (camelCase, e.g. "displayName"); only
// masked fields are touched server-side. Returns the updated resource.
func (c *Client) UpdateUniverse(ctx context.Context, universeID int64, u Universe, updateMask []string) (Universe, error) {
	q := url.Values{"updateMask": {strings.Join(updateMask, ",")}}
	var out Universe
	err := c.doJSON(ctx, "PATCH", fmt.Sprintf(universePathFmt, universeID), q, u, &out)
	return out, err
}
