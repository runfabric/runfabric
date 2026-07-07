package server

import (
	"encoding/json"
	"net/http"

	"github.com/runfabric/runfabric/platform/extensions/application/external"
	manifests "github.com/runfabric/runfabric/platform/extensions/manifest"
	providerloader "github.com/runfabric/runfabric/platform/extensions/registry/loader/providers"
	"github.com/runfabric/runfabric/platform/extensions/registry/resolution"
)

// extensionKindMapping documents how one plugin kind is selected from
// runfabric.yml, so API consumers (dashboards, the PaaS) can render the
// catalog without hardcoding framework knowledge.
type extensionKindMapping struct {
	Kind      manifests.PluginKind
	ConfigKey string
	Default   string
	Note      string
}

// extensionKindMappings is the daemon's authoritative kind → config-key map.
// Keep in step with schemas/runfabric.schema.json and app.Bootstrap.
var extensionKindMappings = []extensionKindMapping{
	{
		Kind:      manifests.KindProvider,
		ConfigKey: "provider.name",
		Note:      "providerOverrides.<key>.name selects alternates for --provider; provider-specific settings live under extensions.<provider-id>",
	},
	{
		Kind:      manifests.KindRouter,
		ConfigKey: "extensions.routerPlugin",
		Default:   "cloudflare",
		Note:      "DNS-sync policy and credentials live under extensions.router.*",
	},
	{
		Kind:      manifests.KindSecretManager,
		ConfigKey: "extensions.secretManagerPlugin",
		Note:      "resolves secret-manager refs (aws-sm://, gcp-sm://, azure-kv://, vault://) used in secrets: values; pin with extensions.secretManagerPluginVersion",
	},
	{
		Kind:      manifests.KindState,
		ConfigKey: "extensions.statePlugin",
		Default:   "local",
		Note:      "backend.kind selects built-in state kinds directly (local, sqlite, postgres, dynamodb, s3, gcs, azblob); pin with extensions.statePluginVersion",
	},
	{
		Kind:      manifests.KindRuntime,
		ConfigKey: "extensions.runtimePlugin",
		Note:      "install-ensure only (with extensions.autoInstallExtensions); the runtime itself is provider.runtime / functions.<name>.runtime",
	},
	{
		Kind:      manifests.KindSimulator,
		ConfigKey: "extensions.simulatorPlugin",
		Note:      "install-ensure only (with extensions.autoInstallExtensions)",
	},
}

type extensionPluginPayload struct {
	ID               string                     `json:"id"`
	Name             string                     `json:"name,omitempty"`
	Description      string                     `json:"description,omitempty"`
	Source           string                     `json:"source"`
	Version          string                     `json:"version,omitempty"`
	Capabilities     []string                   `json:"capabilities,omitempty"`
	SupportsRuntime  []string                   `json:"supportsRuntime,omitempty"`
	SupportsTriggers []string                   `json:"supportsTriggers,omitempty"`
	Credentials      []manifests.CredentialSpec `json:"credentials,omitempty"`
	// Scaffold lets the PaaS drive a metadata-driven New Project flow (entry file,
	// runtime, comment, sample body, and state backend config lines) without
	// hardcoding framework knowledge.
	Scaffold *manifests.ScaffoldSpec `json:"scaffold,omitempty"`
}

type extensionKindPayload struct {
	Kind      string                   `json:"kind"`
	ConfigKey string                   `json:"configKey"`
	Default   string                   `json:"default,omitempty"`
	Note      string                   `json:"note,omitempty"`
	Plugins   []extensionPluginPayload `json:"plugins"`
}

// handleExtensions serves GET /extensions: every plugin kind the engine knows,
// the runfabric.yml key that selects it, and the plugins currently available
// (built-ins merged with external plugins discovered under RUNFABRIC_HOME).
func handleExtensions(w http.ResponseWriter, _ *http.Request) {
	catalog, err := resolution.DiscoverPluginCatalog(external.DiscoverOptions{})
	if err != nil && (catalog == nil || catalog.Registry == nil) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": err.Error()})
		return
	}

	// Provider capability/runtime/trigger data is authoritative in the capability
	// catalog (manifest → live meta → planner matrix), not in the sparse built-in
	// PluginManifest — enrich the provider payload from it so the PaaS gets real
	// runtimes/triggers for the New Project wizard. Best-effort: fall back to
	// manifest values if the catalog is unavailable.
	providerCaps := map[string]providerloader.ProviderDescriptor{}
	if pc, perr := providerloader.NewDefaultProviderCapabilityCatalog(); perr == nil {
		if descs, lerr := pc.ListProviders(); lerr == nil {
			for _, d := range descs {
				providerCaps[d.ID] = d
			}
		}
	}

	kinds := make([]extensionKindPayload, 0, len(extensionKindMappings))
	for _, mapping := range extensionKindMappings {
		entry := extensionKindPayload{
			Kind:      string(mapping.Kind),
			ConfigKey: mapping.ConfigKey,
			Default:   mapping.Default,
			Note:      mapping.Note,
			Plugins:   []extensionPluginPayload{},
		}
		for _, m := range catalog.Registry.List(mapping.Kind) {
			source := m.Source
			if source == "" {
				source = "builtin"
			}
			capabilities, runtimes, triggers := m.Capabilities, m.SupportsRuntime, m.SupportsTriggers
			if mapping.Kind == manifests.KindProvider {
				if d, ok := providerCaps[m.ID]; ok {
					if len(d.Capabilities) > 0 {
						capabilities = d.Capabilities
					}
					if len(d.SupportsRuntime) > 0 {
						runtimes = d.SupportsRuntime
					}
					if len(d.SupportsTriggers) > 0 {
						triggers = d.SupportsTriggers
					}
				}
			}
			entry.Plugins = append(entry.Plugins, extensionPluginPayload{
				ID:               m.ID,
				Name:             m.Name,
				Description:      m.Description,
				Source:           source,
				Version:          m.Version,
				Capabilities:     capabilities,
				SupportsRuntime:  runtimes,
				SupportsTriggers: triggers,
				// Credential surface incl. declarative same-cloud fallbacks —
				// lets the PaaS compute which env keys/headers a plugin needs.
				Credentials: m.Credentials,
				Scaffold:    m.Scaffold,
			})
		}
		kinds = append(kinds, entry)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{"kinds": kinds})
}
