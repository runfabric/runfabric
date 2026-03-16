package config

import (
	"fmt"
	"os"
)

// ResourceProvisionFn is an optional callback to provision a resource (e.g. RDS, ElastiCache) and return its connection string.
// When a resource has "provision: true", this is called first; if it returns ErrNotImplemented or error, binding falls back to connectionStringEnv/connectionString.
type ResourceProvisionFn func(provider, resourceKey string, spec map[string]any) (connectionString string, err error)

// ResolveResourceBindings interprets cfg.Resources and returns a map of env var name -> value
// to inject into function environment at deploy. Supports:
//   - provision: true — optional; when set, provisionFn is called to obtain connection string (e.g. RDS, ElastiCache); fallback to connectionStringEnv/connectionString if not implemented.
//   - connectionStringEnv: name of an env var to read the value from (e.g. DATABASE_URL in CI).
//   - connectionString: literal or ${env:VAR} expression (resolved via resolveEnvStrict).
//
// Each resource entry should have "envVar" (required) and either provision (with provisionFn), "connectionStringEnv", or "connectionString".
func ResolveResourceBindings(cfg *Config, provisionFn ResourceProvisionFn) (map[string]string, error) {
	if cfg == nil || len(cfg.Resources) == 0 {
		return nil, nil
	}
	out := make(map[string]string)
	provider := ""
	if cfg.Provider.Name != "" {
		provider = cfg.Provider.Name
	}
	for name, raw := range cfg.Resources {
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		envVar, _ := m["envVar"].(string)
		if envVar == "" {
			continue
		}
		var value string
		provision, _ := m["provision"].(bool)
		if provision && provisionFn != nil && provider != "" {
			cs, err := provisionFn(provider, name, m)
			if err == nil && cs != "" {
				value = cs
			}
		}
		if value == "" {
			if csEnv, ok := m["connectionStringEnv"].(string); ok && csEnv != "" {
				value = os.Getenv(csEnv)
			} else if cs, ok := m["connectionString"].(string); ok && cs != "" {
				var err error
				value, err = resolveEnvStrict(cs)
				if err != nil {
					return nil, fmt.Errorf("resources.%s.connectionString: %w", name, err)
				}
			}
		}
		if value != "" {
			out[envVar] = value
		}
	}
	return out, nil
}

// EnvVarToResourceKey returns a map from env var name to the resource key that sets it.
// Used to filter resource bindings per function when functions.*.resources is set.
func EnvVarToResourceKey(cfg *Config) map[string]string {
	if cfg == nil || len(cfg.Resources) == 0 {
		return nil
	}
	out := make(map[string]string)
	for name, raw := range cfg.Resources {
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		envVar, _ := m["envVar"].(string)
		if envVar == "" {
			continue
		}
		out[envVar] = name
	}
	return out
}
