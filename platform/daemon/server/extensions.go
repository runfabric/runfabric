package server

import (
	"encoding/json"
	"net/http"

	"github.com/runfabric/runfabric/platform/extensions/application/external"
	manifests "github.com/runfabric/runfabric/platform/extensions/manifest"
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
	ID           string                     `json:"id"`
	Name         string                     `json:"name,omitempty"`
	Description  string                     `json:"description,omitempty"`
	Source       string                     `json:"source"`
	Version      string                     `json:"version,omitempty"`
	Capabilities []string                   `json:"capabilities,omitempty"`
	Credentials  []manifests.CredentialSpec `json:"credentials,omitempty"`
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
			entry.Plugins = append(entry.Plugins, extensionPluginPayload{
				ID:           m.ID,
				Name:         m.Name,
				Description:  m.Description,
				Source:       source,
				Version:      m.Version,
				Capabilities: m.Capabilities,
				// Credential surface incl. declarative same-cloud fallbacks —
				// lets the PaaS compute which env keys/headers a plugin needs.
				Credentials: m.Credentials,
			})
		}
		kinds = append(kinds, entry)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{"kinds": kinds})
}
