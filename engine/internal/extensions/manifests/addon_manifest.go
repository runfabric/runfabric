package manifests

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
	return []*AddonManifest{
		{ID: "sentry", Name: "Sentry", Description: "Error tracking and performance (SENTRY_DSN, etc.)", Permissions: Permissions{Env: true, Network: true}},
		{ID: "datadog", Name: "Datadog", Description: "APM and metrics (DD_API_KEY, DD_APP_KEY, etc.)", Permissions: Permissions{Env: true, Network: true}},
		{ID: "logdrain", Name: "Log drain", Description: "Log drain / external logging (endpoint, API key)", Permissions: Permissions{Env: true, Network: true}},
		{ID: "custom", Name: "Custom", Description: "User-defined add-on (options + secrets)", Permissions: Permissions{Env: true}},
	}
}

// Get returns the addon manifest for id, or nil.
func (a *AddonManifestRegistry) Get(id string) *AddonManifest {
	if a == nil {
		return nil
	}
	return a.manifests[id]
}
