package manifests

import extaddons "github.com/runfabric/runfabric/platform/extensions/addons"

// AddonManifest describes the contract for a RunFabric Addon (JS/TS) for validation and catalog.
type AddonManifest struct {
	ID          string            `json:"id"`
	Name        string            `json:"name,omitempty"`
	Description string            `json:"description,omitempty"`
	Version     string            `json:"version,omitempty"`
	Options     map[string]string `json:"options,omitempty"`
	Secrets     map[string]string `json:"secrets,omitempty"`
	Permissions Permissions       `json:"permissions,omitempty"`
}

// AddonManifestRegistry holds addon manifests for validation.
type AddonManifestRegistry struct {
	manifests map[string]*AddonManifest
}

// NewAddonManifestRegistry returns a registry with built-in addon manifests.
func NewAddonManifestRegistry() *AddonManifestRegistry {
	a := &AddonManifestRegistry{manifests: make(map[string]*AddonManifest)}
	for _, m := range builtinAddonManifests() {
		a.manifests[m.ID] = m
	}
	return a
}

func builtinAddonManifests() []*AddonManifest {
	catalog := extaddons.NewBuiltinAddonsCatalog("")
	out := make([]*AddonManifest, 0, len(catalog))
	for _, e := range catalog {
		out = append(out, &AddonManifest{
			ID:          e.Name,
			Name:        e.Name,
			Description: e.Description,
			Version:     e.Version,
			Permissions: addonPermissions(e.Name),
		})
	}
	return out
}

func addonPermissions(name string) Permissions {
	switch name {
	case "sentry", "datadog", "logdrain":
		return Permissions{Env: true, Network: true}
	default:
		return Permissions{Env: true}
	}
}

// Get returns the addon manifest for id, or nil.
func (a *AddonManifestRegistry) Get(id string) *AddonManifest {
	if a == nil {
		return nil
	}
	return a.manifests[id]
}
