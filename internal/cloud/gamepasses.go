package cloud

import (
	"context"
	"fmt"
)

// Game passes are also legacy-only in Open Cloud. PATH CHOICE (unverified
// against production, same caveat as badges.go): assumed proxied at
// apis.roblox.com under a legacy-game-passes prefix mirroring
// game-passes.roblox.com routes — create under the universe, update by
// game-pass id. Each URL lives in one const so a correction is a one-line
// change.
const (
	gamePassCreatePathFmt = "/legacy-game-passes/v1/universes/%d/game-passes"
	gamePassUpdatePathFmt = "/legacy-game-passes/v1/game-passes/%d"
)

type CreateGamePassRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	// Price in Robux; nil leaves the pass off-sale.
	Price *int64 `json:"price,omitempty"`
}

type UpdateGamePassRequest struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Price       *int64 `json:"price,omitempty"`
	IsForSale   *bool  `json:"isForSale,omitempty"`
}

// GamePass is the legacy game-pass resource (subset deploy cares about).
type GamePass struct {
	GamePassID  int64  `json:"gamePassId"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Price       int64  `json:"price"`
	IsForSale   bool   `json:"isForSale"`
	IconAssetID int64  `json:"iconAssetId"`
}

// CreateGamePass creates a game pass in the universe.
func (c *Client) CreateGamePass(ctx context.Context, universeID int64, req CreateGamePassRequest) (GamePass, error) {
	var g GamePass
	err := c.doJSON(ctx, "POST", fmt.Sprintf(gamePassCreatePathFmt, universeID), nil, req, &g)
	return g, err
}

// UpdateGamePass updates an existing game pass.
func (c *Client) UpdateGamePass(ctx context.Context, gamePassID int64, req UpdateGamePassRequest) (GamePass, error) {
	var g GamePass
	err := c.doJSON(ctx, "PATCH", fmt.Sprintf(gamePassUpdatePathFmt, gamePassID), nil, req, &g)
	return g, err
}
