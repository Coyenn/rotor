// Package config loads and evaluates a project's rotor.config.ts natively,
// with no Node dependency: esbuild (Go API) bundles/transpiles the TypeScript
// to CommonJS, and goja (a pure-Go JavaScript engine) evaluates the result.
//
// The "rotor/config" module imported by config files is virtual: an esbuild
// plugin resolves it in-memory to `export const defineConfig = (c) => c;`.
// Relative imports of other project .ts/.js files are bundled; bare npm
// imports are rejected with a clear error.
package config

import "fmt"

// Config is the typed shape of the object default-exported from
// rotor.config.ts. All sections are optional.
type Config struct {
	Assets *AssetsConfig `json:"assets,omitempty"`
	Deploy *DeployConfig `json:"deploy,omitempty"`

	// Warnings collects non-fatal issues discovered while loading the config
	// (for example unknown top-level keys, which are tolerated for forward
	// compatibility). It is populated by Load and is not part of the config
	// shape itself.
	Warnings []string `json:"-"`
}

// AssetsConfig configures `rotor asset sync`.
type AssetsConfig struct {
	Paths   []string     `json:"paths,omitempty"`
	Output  AssetsOutput `json:"output,omitempty"`
	Creator Creator      `json:"creator,omitempty"`
}

// AssetsOutput is where generated asset modules are written.
type AssetsOutput struct {
	Luau  string `json:"luau,omitempty"`
	Types string `json:"types,omitempty"`
}

// Creator identifies the Roblox creator that owns uploaded assets.
type Creator struct {
	Type string `json:"type,omitempty"` // "user" | "group"
	ID   int64  `json:"id,omitempty"`
}

// DeployConfig configures `rotor deploy`.
type DeployConfig struct {
	Environments map[string]Environment `json:"environments,omitempty"`
}

// Environment is one named deploy target (e.g. "dev", "prod").
type Environment struct {
	UniverseID  int64                  `json:"universeId,omitempty"`
	Places      map[string]PlaceDeploy `json:"places,omitempty"`
	Experience  *ExperienceConfig      `json:"experience,omitempty"`
	Payments    string                 `json:"payments,omitempty"`
	Badges      map[string]Badge       `json:"badges,omitempty"`
	GamePasses  map[string]GamePass    `json:"gamepasses,omitempty"`
	Icon        string                 `json:"icon,omitempty"`        // experience icon image path
	Thumbnails  []string               `json:"thumbnails,omitempty"`  // ordered thumbnail image paths (max 10)
	Products    map[string]Product     `json:"products,omitempty"`    // developer products
	SocialLinks map[string]SocialLink  `json:"socials,omitempty"` // universe social links
}

// PlaceDeploy publishes a built place file to a place id and optionally
// manages the place's metadata.
type PlaceDeploy struct {
	File        string `json:"file,omitempty"`
	PlaceID     int64  `json:"placeId,omitempty"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	MaxPlayers  int32  `json:"maxPlayers,omitempty"`
	VersionType string `json:"versionType,omitempty"` // "saved" | "published" (default)
}

// ExperienceConfig updates universe-level settings.
type ExperienceConfig struct {
	Name           string          `json:"name,omitempty"`
	Description    string          `json:"description,omitempty"`
	Playability    string          `json:"playability,omitempty"` // "public" | "private" | "friends"
	PrivateServers *PrivateServers `json:"privateServers,omitempty"`
}

// PrivateServers enables paid private servers; a nil/absent Price means free
// (0 Robux).
type PrivateServers struct {
	Price *int64 `json:"price,omitempty"`
}

// Badge declares a badge to create or update.
type Badge struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Icon        string `json:"icon,omitempty"`
}

// GamePass declares a game pass to create or update. A nil Price leaves the
// pass not for sale.
type GamePass struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Price       *int64 `json:"price,omitempty"`
	Icon        string `json:"icon,omitempty"`
}

// Product declares a developer product to create or update.
type Product struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Price       int64  `json:"price,omitempty"`
}

// SocialLink declares a universe social link.
type SocialLink struct {
	Title string `json:"title,omitempty"`
	URL   string `json:"url,omitempty"`
	Type  string `json:"type,omitempty"` // facebook|twitter|youtube|twitch|discord|github|guilded
}

// socialLinkTypes is the accepted SocialLink.Type enum.
var socialLinkTypes = map[string]bool{
	"facebook": true, "twitter": true, "youtube": true, "twitch": true,
	"discord": true, "github": true, "guilded": true,
}

// SocialLinkTypeValid reports whether t is an accepted social-link type.
func SocialLinkTypeValid(t string) bool { return socialLinkTypes[t] }

// Validate performs structural validation and returns every problem found.
// It does not touch the network or the filesystem — referenced files (place
// files, icons, thumbnails) are checked at plan/build time, so a config can
// be validated on machines that don't have the built assets.
func (c *Config) Validate() []error {
	var errs []error
	if c.Assets != nil {
		switch c.Assets.Creator.Type {
		case "user", "group":
		default:
			errs = append(errs, fmt.Errorf(
				"assets.creator.type must be %q or %q, got %q",
				"user", "group", c.Assets.Creator.Type))
		}
	}
	if c.Deploy != nil {
		for envName, env := range c.Deploy.Environments {
			for placeName, place := range env.Places {
				if place.File == "" {
					errs = append(errs, fmt.Errorf(
						"deploy.environments.%s.places.%s: file is required",
						envName, placeName))
				}
				if place.PlaceID == 0 {
					errs = append(errs, fmt.Errorf(
						"deploy.environments.%s.places.%s: placeId is required",
						envName, placeName))
				}
				switch place.VersionType {
				case "", "saved", "published":
				default:
					errs = append(errs, fmt.Errorf(
						"deploy.environments.%s.places.%s.versionType must be %q or %q, got %q",
						envName, placeName, "saved", "published", place.VersionType))
				}
				if place.MaxPlayers < 0 {
					errs = append(errs, fmt.Errorf(
						"deploy.environments.%s.places.%s.maxPlayers must be >= 0, got %d",
						envName, placeName, place.MaxPlayers))
				}
			}
			if env.Experience != nil {
				if env.Experience.Playability != "" {
					switch env.Experience.Playability {
					case "public", "private", "friends":
					default:
						errs = append(errs, fmt.Errorf(
							"deploy.environments.%s.experience.playability must be one of %q, %q, %q, got %q",
							envName, "public", "private", "friends", env.Experience.Playability))
					}
				}
				if ps := env.Experience.PrivateServers; ps != nil && ps.Price != nil && *ps.Price < 0 {
					errs = append(errs, fmt.Errorf(
						"deploy.environments.%s.experience.privateServers.price must be >= 0, got %d",
						envName, *ps.Price))
				}
			}
			for passName, pass := range env.GamePasses {
				if pass.Price != nil && *pass.Price < 0 {
					errs = append(errs, fmt.Errorf(
						"deploy.environments.%s.gamepasses.%s.price must be >= 0, got %d",
						envName, passName, *pass.Price))
				}
			}
			if len(env.Thumbnails) > 10 {
				errs = append(errs, fmt.Errorf(
					"deploy.environments.%s.thumbnails: at most 10 thumbnails are allowed, got %d",
					envName, len(env.Thumbnails)))
			}
			for productName, product := range env.Products {
				if product.Price < 0 {
					errs = append(errs, fmt.Errorf(
						"deploy.environments.%s.products.%s.price must be >= 0, got %d",
						envName, productName, product.Price))
				}
			}
			for linkName, link := range env.SocialLinks {
				if !SocialLinkTypeValid(link.Type) {
					errs = append(errs, fmt.Errorf(
						"deploy.environments.%s.socials.%s.type must be one of facebook, twitter, youtube, twitch, discord, github, guilded; got %q",
						envName, linkName, link.Type))
				}
				if link.URL == "" {
					errs = append(errs, fmt.Errorf(
						"deploy.environments.%s.socials.%s: url is required",
						envName, linkName))
				}
			}
		}
	}
	return errs
}
