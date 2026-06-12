package cloud

import (
	"context"
	"fmt"
)

// Badges live on the legacy Open Cloud surface (badges never got a cloud/v2
// resource). PATH CHOICE (unverified against production): the legacy
// badges API is proxied at apis.roblox.com under the legacy-badges prefix,
// mirroring badges.roblox.com/v1 routes — create is
// POST /v1/universes/{universeId}/badges, update is PATCH /v1/badges/{id}.
// If the real prefix differs, only these two consts need fixing.
const (
	badgeCreatePathFmt = "/legacy-badges/v1/universes/%d/badges"
	badgeUpdatePathFmt = "/legacy-badges/v1/badges/%d"
)

// CreateBadgeRequest creates a badge under a universe. PaymentSourceType
// selects who pays the badge fee (1 = User, 2 = Group), matching the legacy
// API's enum.
type CreateBadgeRequest struct {
	Name              string `json:"name"`
	Description       string `json:"description,omitempty"`
	PaymentSourceType int    `json:"paymentSourceType,omitempty"`
}

type UpdateBadgeRequest struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Enabled     *bool  `json:"enabled,omitempty"`
}

// Badge is the legacy badge resource (subset deploy cares about).
type Badge struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
	IconImageID int64  `json:"iconImageId"`
}

// CreateBadge creates a badge in the universe. Note: creating badges past
// the daily free quota costs Robux; callers surface that in plan output.
func (c *Client) CreateBadge(ctx context.Context, universeID int64, req CreateBadgeRequest) (Badge, error) {
	var b Badge
	err := c.doJSON(ctx, "POST", fmt.Sprintf(badgeCreatePathFmt, universeID), nil, req, &b)
	return b, err
}

// UpdateBadge updates name/description/enabled on an existing badge.
func (c *Client) UpdateBadge(ctx context.Context, badgeID int64, req UpdateBadgeRequest) (Badge, error) {
	var b Badge
	err := c.doJSON(ctx, "PATCH", fmt.Sprintf(badgeUpdatePathFmt, badgeID), nil, req, &b)
	return b, err
}
