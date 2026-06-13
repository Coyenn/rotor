package cloud

import (
	"context"
	"fmt"
)

// Developer products are legacy-only in Open Cloud today. PATH CHOICE
// (unverified against production, same caveat as badges.go): assumed proxied
// at apis.roblox.com under the developer-products prefix mirroring
// apis.roblox.com/developer-products v1 routes — create is
// POST /developer-products/v1/universes/{universeId}/developerproducts,
// update is PATCH .../developerproducts/{productId}. Request fields are sent
// as a JSON body (the oldest variant of this API took query parameters; if
// production still requires that, only these methods change). Each URL lives
// in one const so a correction is a one-line change.
const (
	developerProductCreatePathFmt = "/developer-products/v1/universes/%d/developerproducts"
	developerProductUpdatePathFmt = "/developer-products/v1/universes/%d/developerproducts/%d"
)

type CreateDeveloperProductRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	// PriceInRobux is the purchase price; developer products are always for
	// sale (0 is allowed by the API only for some legacy products).
	PriceInRobux int64 `json:"priceInRobux"`
}

type UpdateDeveloperProductRequest struct {
	Name         string `json:"name,omitempty"`
	Description  string `json:"description,omitempty"`
	PriceInRobux int64  `json:"priceInRobux"`
}

// DeveloperProduct is the legacy developer-product resource (subset deploy
// cares about).
type DeveloperProduct struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	PriceInRobux int64  `json:"priceInRobux"`
}

// CreateDeveloperProduct creates a developer product in the universe.
func (c *Client) CreateDeveloperProduct(ctx context.Context, universeID int64, req CreateDeveloperProductRequest) (DeveloperProduct, error) {
	var p DeveloperProduct
	err := c.doJSON(ctx, "POST", fmt.Sprintf(developerProductCreatePathFmt, universeID), nil, req, &p)
	return p, err
}

// UpdateDeveloperProduct updates an existing developer product. (Roblox has
// no developer-product delete; removal is state-only on the deploy side.)
func (c *Client) UpdateDeveloperProduct(ctx context.Context, universeID, productID int64, req UpdateDeveloperProductRequest) (DeveloperProduct, error) {
	var p DeveloperProduct
	err := c.doJSON(ctx, "PATCH", fmt.Sprintf(developerProductUpdatePathFmt, universeID, productID), nil, req, &p)
	return p, err
}
