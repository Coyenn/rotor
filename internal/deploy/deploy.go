// Package deploy is rotor's mantle-style infrastructure-as-code engine for
// Roblox experiences: a resource graph derived from rotor.config.ts, a local
// state file per environment, and terraform-style plan/apply diffing on top
// of the Open Cloud client (internal/cloud).
//
// The engine is the deliverable; resource kinds plug into it via the
// Provider interface. v1 ships place_file (publish an rbxl), place_config
// (name/description PATCH), experience (universe settings PATCH with
// updateMask), badge, and asset (badge-icon upload). More kinds are roadmap
// items, not architecture changes.
package deploy

import (
	"context"
	"encoding/json"
	"io"
	"path/filepath"

	"rotor/internal/cloud"
)

// Resource kinds implemented in v1. The state-file key for a resource is
// "<kind>/<name>".
const (
	KindPlaceFile   = "place_file"   // publish a built .rbxl to a place id
	KindPlaceConfig = "place_config" // PATCH place name/description
	KindExperience  = "experience"   // PATCH universe settings (updateMask)
	KindBadge       = "badge"        // create/update a badge
	KindAsset       = "asset"        // upload a file (badge icons) as an asset
)

// ResourceRef names another resource in the graph, for DependsOn edges and
// output lookups.
type ResourceRef struct {
	Kind string
	Name string
}

// Key is the canonical "<kind>/<name>" identity used in state files and plan
// output.
func (r ResourceRef) Key() string { return r.Kind + "/" + r.Name }

func (r ResourceRef) String() string { return r.Key() }

// Resource is one desired-state node in the deployment graph. Inputs must be
// JSON-marshalable: they are canonically hashed for drift detection and
// handed to the kind's Provider on create/update.
type Resource struct {
	Kind      string
	Name      string
	DependsOn []ResourceRef
	Inputs    any
}

// Ref returns the resource's graph identity.
func (r Resource) Ref() ResourceRef { return ResourceRef{Kind: r.Kind, Name: r.Name} }

// CloudClient is the slice of internal/cloud that providers use. It is an
// interface so engine tests run against an in-memory fake; *cloud.Client
// satisfies it.
type CloudClient interface {
	UpdateUniverse(ctx context.Context, universeID int64, u cloud.Universe, updateMask []string) (cloud.Universe, error)
	UpdatePlace(ctx context.Context, universeID, placeID int64, p cloud.Place, updateMask []string) (cloud.Place, error)
	PublishPlaceVersion(ctx context.Context, universeID, placeID int64, versionType string, body io.Reader) (int64, error)
	CreateBadge(ctx context.Context, universeID int64, req cloud.CreateBadgeRequest) (cloud.Badge, error)
	UpdateBadge(ctx context.Context, badgeID int64, req cloud.UpdateBadgeRequest) (cloud.Badge, error)
	CreateAsset(ctx context.Context, req cloud.CreateAssetRequest, fileName string, file io.Reader) (operationPath string, err error)
	PollOperation(ctx context.Context, path string, into any) error
}

// Compile-time check that the real client implements the provider-facing
// interface.
var _ CloudClient = (*cloud.Client)(nil)

// Ctx carries everything a Provider needs beyond its own inputs: the cloud
// client, the target universe, the project root for resolving relative file
// paths, and a lookup into other resources' outputs (e.g. a badge reading
// the asset id its icon upload produced).
type Ctx struct {
	Cloud      CloudClient
	UniverseID int64
	ProjectDir string

	// Output reads one output value of another (already-applied) resource.
	// During Apply it reflects the state as of the current step, so a
	// resource sees outputs from dependencies applied earlier in the run.
	Output func(ref ResourceRef, key string) (any, bool)
}

// ResolvePath resolves a config-relative file path against the project root.
func (c *Ctx) ResolvePath(p string) string {
	if p == "" || filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(c.ProjectDir, p)
}

// Provider implements one resource kind. Create and Update return the
// resource's outputs (ids, version numbers), which persist to state and are
// readable by dependent resources via Ctx.Output. prior is the resource's
// previous state entry (nil on first create).
type Provider interface {
	Create(ctx context.Context, c *Ctx, inputs any, prior *StateEntry) (map[string]any, error)
	Update(ctx context.Context, c *Ctx, inputs any, prior *StateEntry) (map[string]any, error)
	Delete(ctx context.Context, c *Ctx, prior *StateEntry) error
}

// DefaultProviders returns the registry of v1 resource kinds.
func DefaultProviders() map[string]Provider {
	return map[string]Provider{
		KindPlaceFile:   placeFileProvider{},
		KindPlaceConfig: placeConfigProvider{},
		KindExperience:  experienceProvider{},
		KindBadge:       badgeProvider{},
		KindAsset:       assetProvider{},
	}
}

// decodeInputs converts a resource's Inputs (typed struct from the graph
// builder, or a plain map in tests) into the provider's concrete input type
// via a JSON round-trip.
func decodeInputs[T any](inputs any) (T, error) {
	if t, ok := inputs.(T); ok {
		return t, nil
	}
	var out T
	data, err := json.Marshal(inputs)
	if err != nil {
		return out, err
	}
	err = json.Unmarshal(data, &out)
	return out, err
}

// OutputInt64 coerces a state output value to int64. Outputs round-trip
// through JSON, so a value stored as int64 comes back as float64 after a
// state reload; both (plus json.Number) are accepted.
func OutputInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case int64:
		return n, true
	case int:
		return int64(n), true
	case float64:
		return int64(n), true
	case json.Number:
		i, err := n.Int64()
		return i, err == nil
	}
	return 0, false
}
