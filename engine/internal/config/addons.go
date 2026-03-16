package config

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// AddonCatalogEntry describes an add-on in the catalog (for list/validate). Name and optional Version/Description.
type AddonCatalogEntry struct {
	Name        string `json:"name"`
	Version     string `json:"version,omitempty"`
	Description string `json:"description,omitempty"`
}

// AddonCatalog returns the built-in list of known add-ons. Used by `runfabric addons list` and optional validation.
func AddonCatalog() []AddonCatalogEntry {
	return []AddonCatalogEntry{
		{Name: "sentry", Description: "Error tracking and performance (SENTRY_DSN, etc.)"},
		{Name: "datadog", Description: "APM and metrics (DD_API_KEY, DD_APP_KEY, etc.)"},
		{Name: "logdrain", Description: "Log drain / external logging (endpoint, API key)"},
		{Name: "custom", Description: "User-defined add-on (options + secrets)"},
	}
}

// FetchAddonCatalog fetches addon catalog entries from the given URL. Expects a JSON array of objects with "name", optional "version", "description".
// Returns nil on error or invalid response (caller can fall back to built-in only).
func FetchAddonCatalog(url string) []AddonCatalogEntry {
	if url == "" {
		return nil
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	var entries []AddonCatalogEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil
	}
	return entries
}

// MergeAddonCatalogs merges built-in and optionally fetched catalog. Fetched entries are appended; duplicate names are not deduplicated.
func MergeAddonCatalogs(builtin, fetched []AddonCatalogEntry) []AddonCatalogEntry {
	out := make([]AddonCatalogEntry, 0, len(builtin)+len(fetched))
	out = append(out, builtin...)
	out = append(out, fetched...)
	return out
}

// ValidateAddons checks addon names and secret keys. If catalog is used, names can be validated; optional.
func ValidateAddons(cfg *Config) error {
	if cfg == nil || len(cfg.Addons) == 0 {
		return nil
	}
	catalogNames := make(map[string]struct{})
	for _, e := range AddonCatalog() {
		catalogNames[e.Name] = struct{}{}
	}
	for key, addon := range cfg.Addons {
		name := addon.Name
		if name == "" {
			name = key
		}
		if name != key && strings.TrimSpace(name) == "" {
			return fmt.Errorf("addons.%s: name cannot be empty when key is used", key)
		}
		for envVar := range addon.Secrets {
			if strings.TrimSpace(envVar) == "" {
				return fmt.Errorf("addons.%s.secrets: env var name cannot be empty", key)
			}
		}
		_ = catalogNames // optional: if we want to restrict to catalog only, check name is in catalogNames
	}
	return nil
}

// ResolveAddonBindings resolves addon secrets (env var -> ref) and returns a map of env var name -> value
// to inject into function environment at deploy. Refs can be "${env:VAR}" or a key into config.Secrets (which may hold "${env:VAR}").
func ResolveAddonBindings(cfg *Config) (map[string]string, error) {
	return ResolveAddonBindingsForKeys(cfg, nil)
}

// ResolveAddonBindingsForKeys resolves addon secrets only for the given addon keys. If addonKeys is nil or empty, all addons are resolved.
// Used for per-function addon attachment: when functions.<name>.addons is set, only those addons' secrets are injected for that function.
func ResolveAddonBindingsForKeys(cfg *Config, addonKeys []string) (map[string]string, error) {
	if cfg == nil || len(cfg.Addons) == 0 {
		return nil, nil
	}
	allowed := make(map[string]struct{})
	for _, k := range addonKeys {
		allowed[k] = struct{}{}
	}
	out := make(map[string]string)
	for key, addon := range cfg.Addons {
		if len(addonKeys) > 0 {
			if _, ok := allowed[key]; !ok {
				continue
			}
		}
		for envVar, ref := range addon.Secrets {
			if ref == "" {
				continue
			}
			if cfg.Secrets != nil {
				if v, ok := cfg.Secrets[ref]; ok && v != "" {
					ref = v
				}
			}
			value, err := resolveEnvStrict(ref)
			if err != nil {
				return nil, fmt.Errorf("addons.%s.secrets.%s: %w", key, envVar, err)
			}
			out[envVar] = value
		}
	}
	return out, nil
}
