package cloud

import (
	"context"
	"fmt"
)

// Universe social links are legacy-only in Open Cloud (cloud/v2 Universe
// exposes per-network socialLink fields, but not the generic typed list this
// API manages). PATH CHOICE (unverified against production, same caveat as
// badges.go): assumed proxied at apis.roblox.com under a legacy-develop
// prefix mirroring develop.roblox.com/v1 routes — create is
// POST /legacy-develop/v1/universes/{universeId}/social-links, update is
// PATCH .../social-links/{id}, delete is DELETE .../social-links/{id}. Each
// URL lives in one const so a correction is a one-line change.
const (
	socialLinkCreatePathFmt = "/legacy-develop/v1/universes/%d/social-links"
	socialLinkByIDPathFmt   = "/legacy-develop/v1/universes/%d/social-links/%d"
)

// SocialLinkRequest creates or updates a universe social link. Type is the
// API's PascalCase enum ("Facebook", "Twitter", "YouTube", "Twitch",
// "Discord", "GitHub", "Guilded"); callers map their own spellings before
// calling.
type SocialLinkRequest struct {
	Title string `json:"title"`
	URL   string `json:"url"`
	Type  string `json:"type"`
}

// SocialLink is the legacy social-link resource (subset deploy cares about).
type SocialLink struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
	URL   string `json:"url"`
	Type  string `json:"type"`
}

// CreateSocialLink adds a social link to the universe.
func (c *Client) CreateSocialLink(ctx context.Context, universeID int64, req SocialLinkRequest) (SocialLink, error) {
	var s SocialLink
	err := c.doJSON(ctx, "POST", fmt.Sprintf(socialLinkCreatePathFmt, universeID), nil, req, &s)
	return s, err
}

// UpdateSocialLink updates an existing social link.
func (c *Client) UpdateSocialLink(ctx context.Context, universeID, linkID int64, req SocialLinkRequest) (SocialLink, error) {
	var s SocialLink
	err := c.doJSON(ctx, "PATCH", fmt.Sprintf(socialLinkByIDPathFmt, universeID, linkID), nil, req, &s)
	return s, err
}

// DeleteSocialLink removes a social link from the universe.
func (c *Client) DeleteSocialLink(ctx context.Context, universeID, linkID int64) error {
	return c.do(ctx, "DELETE", fmt.Sprintf(socialLinkByIDPathFmt, universeID, linkID), nil, "", nil, nil)
}
