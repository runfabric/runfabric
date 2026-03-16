package config

import (
	"fmt"
	"strings"
)

// ApplyProviderOverride replaces cfg.Provider with the entry from cfg.ProviderOverrides[key].
// Use when the CLI is run with --provider <key>. If key is empty, no change. Returns error if key is set but not found.
func ApplyProviderOverride(cfg *Config, key string) error {
	if key == "" {
		return nil
	}
	if cfg.ProviderOverrides == nil {
		return fmt.Errorf("--provider %q requires providerOverrides in runfabric.yml (e.g. providerOverrides.%s: { name: ..., runtime: ... })", key, key)
	}
	override, ok := cfg.ProviderOverrides[key]
	if !ok || strings.TrimSpace(override.Name) == "" {
		names := make([]string, 0, len(cfg.ProviderOverrides))
		for k := range cfg.ProviderOverrides {
			names = append(names, k)
		}
		return fmt.Errorf("provider %q not in providerOverrides (available: %v)", key, names)
	}
	cfg.Provider = override
	if override.Backend != nil {
		cfg.Backend = override.Backend
	}
	return nil
}

// ListProviderKeys returns the list of keys in ProviderOverrides for use in errors and help.
func ListProviderKeys(cfg *Config) []string {
	if cfg.ProviderOverrides == nil || len(cfg.ProviderOverrides) == 0 {
		return nil
	}
	keys := make([]string, 0, len(cfg.ProviderOverrides))
	for k := range cfg.ProviderOverrides {
		keys = append(keys, k)
	}
	return keys
}
