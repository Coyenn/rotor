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
	UniverseID int64                  `json:"universeId,omitempty"`
	Places     map[string]PlaceDeploy `json:"places,omitempty"`
	Experience *ExperienceConfig      `json:"experience,omitempty"`
	Payments   string                 `json:"payments,omitempty"`
	Badges     map[string]Badge       `json:"badges,omitempty"`
}

// PlaceDeploy publishes a built place file to a place id.
type PlaceDeploy struct {
	File    string `json:"file,omitempty"`
	PlaceID int64  `json:"placeId,omitempty"`
}

// ExperienceConfig updates universe-level settings.
type ExperienceConfig struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Playability string `json:"playability,omitempty"` // "public" | "private" | "friends"
}

// Badge declares a badge to create or update.
type Badge struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Icon        string `json:"icon,omitempty"`
}

// Validate performs structural validation and returns every problem found.
// It does not touch the network or the filesystem.
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
			}
			if env.Experience != nil && env.Experience.Playability != "" {
				switch env.Experience.Playability {
				case "public", "private", "friends":
				default:
					errs = append(errs, fmt.Errorf(
						"deploy.environments.%s.experience.playability must be one of %q, %q, %q, got %q",
						envName, "public", "private", "friends", env.Experience.Playability))
				}
			}
		}
	}
	return errs
}
