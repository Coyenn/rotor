package deploy

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"rotor/internal/cloud"
	"rotor/internal/config"
)

// DefaultVersionType is how place files are published when the config does
// not say otherwise (PlaceDeploy.versionType: "saved" | "published").
const DefaultVersionType = cloud.VersionTypePublished

// BuildResources turns one config environment into the desired resource
// graph. It reads local files (to content-hash place files, icons, and
// thumbnails) but never touches the network. Returns the resources and the
// environment's universe id.
//
// Mapping:
//   - each places entry        -> place_file/<name> (file + content hash + versionType),
//     plus place_config/<name> when name/description/maxPlayers are set
//   - experience / payments    -> experience/universe (universe PATCH inputs)
//   - each badges entry        -> badge/<name>, depending on asset/<icon path>
//     when an icon is set (the asset resource uploads the icon)
//   - each gamePasses entry    -> game_pass/<name>, same icon-asset pattern;
//     icon files shared between badges and passes dedupe to one asset
//   - icon                     -> experience_icon/icon (content-hashed)
//   - thumbnails               -> experience_thumbnails/thumbnails (one
//     resource over the ordered, content-hashed list)
//   - each products entry      -> developer_product/<name>
//   - each socialLinks entry   -> social_link/<name>
func BuildResources(projectDir string, cfg *config.Config, envName string) ([]Resource, int64, error) {
	if cfg == nil || cfg.Deploy == nil || len(cfg.Deploy.Environments) == 0 {
		return nil, 0, fmt.Errorf("deploy: rotor.config.ts has no deploy.environments section")
	}
	env, ok := cfg.Deploy.Environments[envName]
	if !ok {
		names := make([]string, 0, len(cfg.Deploy.Environments))
		for n := range cfg.Deploy.Environments {
			names = append(names, n)
		}
		sort.Strings(names)
		return nil, 0, fmt.Errorf("deploy: environment %q not found in rotor.config.ts (have: %s)",
			envName, strings.Join(names, ", "))
	}
	if env.UniverseID == 0 {
		return nil, 0, fmt.Errorf("deploy: environment %q has no universeId", envName)
	}

	var resources []Resource

	// Places, sorted for deterministic graphs.
	placeNames := sortedKeys(env.Places)
	for _, name := range placeNames {
		p := env.Places[name]
		hash, err := HashFile(resolvePath(projectDir, p.File))
		if err != nil {
			return nil, 0, fmt.Errorf("deploy: place %q: hashing %s: %w", name, p.File, err)
		}
		versionType, err := versionTypeFor(p.VersionType)
		if err != nil {
			return nil, 0, fmt.Errorf("deploy: place %q: %w", name, err)
		}
		resources = append(resources, Resource{
			Kind: KindPlaceFile,
			Name: name,
			Inputs: PlaceFileInputs{
				PlaceID:     p.PlaceID,
				File:        p.File,
				FileHash:    hash,
				VersionType: versionType,
			},
		})
		// Place metadata, only when the config manages any of it.
		if p.Name != "" || p.Description != "" || p.MaxPlayers > 0 {
			resources = append(resources, Resource{
				Kind: KindPlaceConfig,
				Name: name,
				Inputs: PlaceConfigInputs{
					PlaceID:     p.PlaceID,
					Name:        p.Name,
					Description: p.Description,
					ServerSize:  p.MaxPlayers,
				},
			})
		}
	}

	// Universe settings.
	if env.Experience != nil || env.Payments != "" {
		in := ExperienceInputs{Payments: env.Payments}
		if env.Experience != nil {
			in.Name = env.Experience.Name
			in.Description = env.Experience.Description
			in.Playability = env.Experience.Playability
			if ps := env.Experience.PrivateServers; ps != nil {
				price := int64(0)
				if ps.Price != nil {
					price = *ps.Price
				}
				in.PrivateServerPrice = &price
			}
		}
		resources = append(resources, Resource{Kind: KindExperience, Name: "universe", Inputs: in})
	}

	// Badge + game-pass icons become asset resources, deduplicated across
	// both kinds: a file shared by a badge and a pass uploads once.
	var creatorType string
	var creatorID int64
	if cfg.Assets != nil {
		creatorType = cfg.Assets.Creator.Type
		creatorID = cfg.Assets.Creator.ID
	}
	seenIcons := map[string]bool{}
	// iconAsset registers (once) the asset resource for an icon file and
	// returns the asset's graph name; owner names the referencing resource
	// for error messages.
	iconAsset := func(owner, icon string) (string, error) {
		iconName := filepath.ToSlash(icon)
		if seenIcons[iconName] {
			return iconName, nil
		}
		seenIcons[iconName] = true
		iconPath := resolvePath(projectDir, icon)
		hash, err := HashFile(iconPath)
		if err != nil {
			return "", fmt.Errorf("deploy: %s: hashing icon %s: %w", owner, icon, err)
		}
		assetType, err := assetTypeForFile(icon)
		if err != nil {
			return "", fmt.Errorf("deploy: %s: %w", owner, err)
		}
		base := filepath.Base(iconPath)
		resources = append(resources, Resource{
			Kind: KindAsset,
			Name: iconName,
			Inputs: AssetInputs{
				File:        icon,
				FileHash:    hash,
				AssetType:   assetType,
				DisplayName: strings.TrimSuffix(base, filepath.Ext(base)),
				CreatorType: creatorType,
				CreatorID:   creatorID,
			},
		})
		return iconName, nil
	}

	for _, name := range sortedKeys(env.Badges) {
		b := env.Badges[name]
		var deps []ResourceRef
		iconName := ""
		if b.Icon != "" {
			var err error
			if iconName, err = iconAsset(fmt.Sprintf("badge %q", name), b.Icon); err != nil {
				return nil, 0, err
			}
			deps = append(deps, ResourceRef{Kind: KindAsset, Name: iconName})
		}
		resources = append(resources, Resource{
			Kind:      KindBadge,
			Name:      name,
			DependsOn: deps,
			Inputs:    BadgeInputs{Name: b.Name, Description: b.Description, Icon: iconName},
		})
	}

	// Game passes (same icon-asset pattern as badges).
	for _, name := range sortedKeys(env.GamePasses) {
		g := env.GamePasses[name]
		var deps []ResourceRef
		iconName := ""
		if g.Icon != "" {
			var err error
			if iconName, err = iconAsset(fmt.Sprintf("game pass %q", name), g.Icon); err != nil {
				return nil, 0, err
			}
			deps = append(deps, ResourceRef{Kind: KindAsset, Name: iconName})
		}
		resources = append(resources, Resource{
			Kind:      KindGamePass,
			Name:      name,
			DependsOn: deps,
			Inputs:    GamePassInputs{Name: g.Name, Description: g.Description, Price: g.Price, Icon: iconName},
		})
	}

	// Experience icon (direct upload; not an asset resource).
	if env.Icon != "" {
		hash, err := HashFile(resolvePath(projectDir, env.Icon))
		if err != nil {
			return nil, 0, fmt.Errorf("deploy: icon: hashing %s: %w", env.Icon, err)
		}
		resources = append(resources, Resource{
			Kind:   KindExperienceIcon,
			Name:   "icon",
			Inputs: ExperienceIconInputs{File: env.Icon, FileHash: hash},
		})
	}

	// Thumbnails: one resource over the ordered set.
	if len(env.Thumbnails) > 0 {
		in := ExperienceThumbnailsInputs{
			Files:      make([]string, 0, len(env.Thumbnails)),
			FileHashes: make([]string, 0, len(env.Thumbnails)),
		}
		for _, file := range env.Thumbnails {
			hash, err := HashFile(resolvePath(projectDir, file))
			if err != nil {
				return nil, 0, fmt.Errorf("deploy: thumbnails: hashing %s: %w", file, err)
			}
			in.Files = append(in.Files, file)
			in.FileHashes = append(in.FileHashes, hash)
		}
		resources = append(resources, Resource{Kind: KindExperienceThumbnails, Name: "thumbnails", Inputs: in})
	}

	// Developer products.
	for _, name := range sortedKeys(env.Products) {
		p := env.Products[name]
		resources = append(resources, Resource{
			Kind:   KindDeveloperProduct,
			Name:   name,
			Inputs: DeveloperProductInputs{Name: p.Name, Description: p.Description, Price: p.Price},
		})
	}

	// Social links.
	for _, name := range sortedKeys(env.SocialLinks) {
		l := env.SocialLinks[name]
		resources = append(resources, Resource{
			Kind:   KindSocialLink,
			Name:   name,
			Inputs: SocialLinkInputs{Title: l.Title, URL: l.URL, Type: l.Type},
		})
	}

	return resources, env.UniverseID, nil
}

// versionTypeFor maps the config's lowercase versionType onto the publish
// API's enum, defaulting to Published.
func versionTypeFor(v string) (string, error) {
	switch v {
	case "":
		return DefaultVersionType, nil
	case "saved":
		return cloud.VersionTypeSaved, nil
	case "published":
		return cloud.VersionTypePublished, nil
	default:
		return "", fmt.Errorf("invalid versionType %q (want \"saved\" or \"published\")", v)
	}
}

// assetTypeForFile maps an upload's extension to the Open Cloud asset type.
func assetTypeForFile(name string) (string, error) {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png", ".jpg", ".jpeg", ".bmp", ".tga":
		return "Decal", nil
	case ".ogg", ".mp3":
		return "Audio", nil
	default:
		return "", fmt.Errorf("unsupported asset file type %q (want png/jpg/jpeg/bmp/tga/ogg/mp3)", filepath.Ext(name))
	}
}

func resolvePath(projectDir, p string) string {
	if p == "" || filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(projectDir, p)
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// topoSort orders resources so every resource comes after everything it
// depends on. Deterministic: within each dependency level, resources sort by
// (kind, name). Duplicate identities, references to resources outside the
// graph, and dependency cycles are errors.
func topoSort(resources []Resource) ([]Resource, error) {
	byKey := make(map[string]*Resource, len(resources))
	for i := range resources {
		key := resources[i].Ref().Key()
		if _, dup := byKey[key]; dup {
			return nil, fmt.Errorf("deploy: duplicate resource %s", key)
		}
		byKey[key] = &resources[i]
	}

	indegree := make(map[string]int, len(resources))
	dependents := make(map[string][]string, len(resources))
	for i := range resources {
		key := resources[i].Ref().Key()
		indegree[key] += 0
		for _, dep := range resources[i].DependsOn {
			depKey := dep.Key()
			if _, ok := byKey[depKey]; !ok {
				return nil, fmt.Errorf("deploy: %s depends on %s, which is not in the graph", key, depKey)
			}
			indegree[key]++
			dependents[depKey] = append(dependents[depKey], key)
		}
	}

	// Kahn's algorithm by levels; sorting each level keeps the order stable
	// regardless of config map iteration order.
	var ready []string
	for key, deg := range indegree {
		if deg == 0 {
			ready = append(ready, key)
		}
	}
	ordered := make([]Resource, 0, len(resources))
	for len(ready) > 0 {
		sort.Strings(ready)
		level := ready
		ready = nil
		for _, key := range level {
			ordered = append(ordered, *byKey[key])
			for _, dep := range dependents[key] {
				if indegree[dep]--; indegree[dep] == 0 {
					ready = append(ready, dep)
				}
			}
		}
	}
	if len(ordered) != len(resources) {
		var stuck []string
		for key, deg := range indegree {
			if deg > 0 {
				stuck = append(stuck, key)
			}
		}
		sort.Strings(stuck)
		return nil, fmt.Errorf("deploy: dependency cycle involving %s", strings.Join(stuck, ", "))
	}
	return ordered, nil
}
