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
// not say otherwise. "Saved" support is a config follow-up.
const DefaultVersionType = cloud.VersionTypePublished

// BuildResources turns one config environment into the desired resource
// graph. It reads local files (to content-hash place files and badge icons)
// but never touches the network. Returns the resources and the environment's
// universe id.
//
// v1 mapping:
//   - each places entry      -> place_file/<name> (file + content hash + versionType)
//   - experience / payments  -> experience/universe (universe PATCH inputs)
//   - each badges entry      -> badge/<name>, depending on asset/<icon path>
//     when an icon is set (the asset resource uploads the icon)
//
// place_config resources are emitted only when the config carries place
// name/description fields; config.PlaceDeploy has none yet, so none are
// generated today (the kind and provider exist for direct use and for the
// config fields to come).
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
		resources = append(resources, Resource{
			Kind: KindPlaceFile,
			Name: name,
			Inputs: PlaceFileInputs{
				PlaceID:     p.PlaceID,
				File:        p.File,
				FileHash:    hash,
				VersionType: DefaultVersionType,
			},
		})
	}

	// Universe settings.
	if env.Experience != nil || env.Payments != "" {
		in := ExperienceInputs{Payments: env.Payments}
		if env.Experience != nil {
			in.Name = env.Experience.Name
			in.Description = env.Experience.Description
			in.Playability = env.Experience.Playability
		}
		resources = append(resources, Resource{Kind: KindExperience, Name: "universe", Inputs: in})
	}

	// Badges (+ deduplicated icon asset resources).
	var creatorType string
	var creatorID int64
	if cfg.Assets != nil {
		creatorType = cfg.Assets.Creator.Type
		creatorID = cfg.Assets.Creator.ID
	}
	seenIcons := map[string]bool{}
	for _, name := range sortedKeys(env.Badges) {
		b := env.Badges[name]
		var deps []ResourceRef
		iconName := ""
		if b.Icon != "" {
			iconName = filepath.ToSlash(b.Icon)
			if !seenIcons[iconName] {
				seenIcons[iconName] = true
				iconPath := resolvePath(projectDir, b.Icon)
				hash, err := HashFile(iconPath)
				if err != nil {
					return nil, 0, fmt.Errorf("deploy: badge %q: hashing icon %s: %w", name, b.Icon, err)
				}
				assetType, err := assetTypeForFile(b.Icon)
				if err != nil {
					return nil, 0, fmt.Errorf("deploy: badge %q: %w", name, err)
				}
				base := filepath.Base(iconPath)
				resources = append(resources, Resource{
					Kind: KindAsset,
					Name: iconName,
					Inputs: AssetInputs{
						File:        b.Icon,
						FileHash:    hash,
						AssetType:   assetType,
						DisplayName: strings.TrimSuffix(base, filepath.Ext(base)),
						CreatorType: creatorType,
						CreatorID:   creatorID,
					},
				})
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

	return resources, env.UniverseID, nil
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
