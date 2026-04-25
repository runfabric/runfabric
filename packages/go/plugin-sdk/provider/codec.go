package provider

import (
	"encoding/json"
	"fmt"
)

// DecodeConfig deserializes the schema-free Config map into a typed struct T
// using a JSON round-trip. T should be a struct with `json:"..."` tags matching
// the runfabric.yml field names (camelCase).
//
// Usage in a provider:
//
//	type MyConfig struct {
//	    ServiceType string `json:"serviceType,omitempty"`
//	    Namespace   string `json:"namespace,omitempty"`
//	    Image       string `json:"image,omitempty"`
//	}
//
//	func (r Runner) Deploy(ctx context.Context, req DeployRequest) (*DeployResult, error) {
//	    var cfg MyConfig
//	    if err := provider.DecodeProviderConfig(req.Config, &cfg); err != nil {
//	        return nil, err
//	    }
//	    // use cfg.ServiceType, cfg.Namespace, etc.
//	}
func DecodeConfig[T any](cfg Config, out *T) error {
	if out == nil {
		return fmt.Errorf("DecodeConfig: out must not be nil")
	}
	raw := providerSection(cfg)
	if len(raw) == 0 {
		return nil
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return fmt.Errorf("DecodeConfig: marshal: %w", err)
	}
	if err := json.Unmarshal(b, out); err != nil {
		return fmt.Errorf("DecodeConfig: unmarshal into %T: %w", *out, err)
	}
	return nil
}

// DecodeSection deserializes an arbitrary top-level section of Config (e.g.
// "deploy", "extensions") into a typed struct T.
//
//	var deploy DeploySection
//	provider.DecodeSection(req.Config, "deploy", &deploy)
func DecodeSection[T any](cfg Config, section string, out *T) error {
	if out == nil {
		return fmt.Errorf("DecodeSection: out must not be nil")
	}
	raw := asMap(first(cfg, section))
	if len(raw) == 0 {
		return nil
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return fmt.Errorf("DecodeSection %q: marshal: %w", section, err)
	}
	if err := json.Unmarshal(b, out); err != nil {
		return fmt.Errorf("DecodeSection %q: unmarshal into %T: %w", section, *out, err)
	}
	return nil
}

// providerSection returns the provider sub-map from cfg, checking both
// "provider" and "Provider" keys (the codec round-trip may capitalise keys).
func providerSection(cfg Config) map[string]any {
	return asMap(first(cfg, "provider", "Provider"))
}
