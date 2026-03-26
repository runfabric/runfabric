package providers

import (
	"fmt"
	"sort"
	"strings"

	providercontract "github.com/runfabric/runfabric/internal/provider/contracts"
	"github.com/runfabric/runfabric/platform/extensions/application/external"
	"github.com/runfabric/runfabric/platform/extensions/registry/resolution"
	planner "github.com/runfabric/runfabric/platform/planner/engine"
)

// ProviderDescriptor is the unified provider capability view used by CLI commands.
type ProviderDescriptor struct {
	ID                string
	Source            string
	Capabilities      []string
	SupportsRuntime   []string
	SupportsTriggers  []string
	SupportsResources []string
}

// ProviderCapabilityCatalog decouples provider capability queries from planner static maps.
// Commands can use this single interface to support both internal and installed external providers.
type ProviderCapabilityCatalog interface {
	ListProviders() ([]ProviderDescriptor, error)
	SupportedTriggers(providerID string) ([]string, error)
	SupportsTrigger(providerID, trigger string) (bool, error)
}

// NewProviderCapabilityCatalog builds a provider capability catalog from the shared extension boundary.
func NewProviderCapabilityCatalog(opts LoadOptions) (ProviderCapabilityCatalog, error) {
	boundary, err := LoadBoundary(opts)
	if err != nil {
		return nil, err
	}
	return &boundaryProviderCatalog{boundary: boundary}, nil
}

// NewDefaultProviderCapabilityCatalog includes installed external plugins and honors prefer-external env.
func NewDefaultProviderCapabilityCatalog() (ProviderCapabilityCatalog, error) {
	return NewProviderCapabilityCatalog(LoadOptions{
		IncludeExternal: true,
		PreferExternal:  external.PreferExternalFromEnv(),
	})
}

type boundaryProviderCatalog struct {
	boundary *resolution.Boundary
}

func (c *boundaryProviderCatalog) ListProviders() ([]ProviderDescriptor, error) {
	if c.boundary == nil {
		return nil, fmt.Errorf("provider capability catalog is not initialized")
	}
	reg := c.boundary.ProviderRegistry()
	if reg == nil {
		return nil, fmt.Errorf("provider registry is not available")
	}
	plugs := c.boundary.PluginRegistry()

	metas := reg.List()
	out := make([]ProviderDescriptor, 0, len(metas))
	for _, meta := range metas {
		id := strings.TrimSpace(meta.Name)
		if id == "" {
			continue
		}
		p, ok := reg.Get(id)
		if !ok {
			continue
		}

		source := "internal"
		manifestCapabilities := []string(nil)
		manifestRuntime := []string(nil)
		manifestTriggers := []string(nil)
		manifestResources := []string(nil)
		if plugs != nil {
			if manifest := plugs.Get(id); manifest != nil {
				if strings.TrimSpace(manifest.Source) != "" {
					source = strings.TrimSpace(manifest.Source)
				}
				manifestCapabilities = normalizedUnique(manifest.Capabilities)
				manifestRuntime = normalizedUnique(manifest.SupportsRuntime)
				manifestTriggers = normalizedUnique(manifest.SupportsTriggers)
				manifestResources = normalizedUnique(manifest.SupportsResources)
			}
		}

		triggers := manifestTriggers
		if len(triggers) == 0 {
			triggers = normalizedUnique(meta.SupportsTriggers)
		}
		if len(triggers) == 0 {
			triggers = normalizedUnique(planner.SupportedTriggers(id))
		}

		capabilities := manifestCapabilities
		if len(capabilities) == 0 {
			capabilities = normalizedUnique(meta.Capabilities)
		}
		if _, ok := p.(providercontract.ObservabilityCapable); ok {
			capabilities = appendUnique(capabilities, "observability")
		}
		if _, ok := p.(providercontract.DevStreamCapable); ok {
			capabilities = appendUnique(capabilities, "dev-stream")
		}
		if _, ok := p.(providercontract.RecoveryCapable); ok {
			capabilities = appendUnique(capabilities, "recovery")
		}
		if _, ok := p.(providercontract.OrchestrationCapable); ok {
			capabilities = appendUnique(capabilities, "orchestration")
		}

		out = append(out, ProviderDescriptor{
			ID:           id,
			Source:       source,
			Capabilities: capabilities,
			SupportsRuntime: func() []string {
				if len(manifestRuntime) > 0 {
					return manifestRuntime
				}
				return normalizedUnique(meta.SupportsRuntime)
			}(),
			SupportsTriggers: triggers,
			SupportsResources: func() []string {
				if len(manifestResources) > 0 {
					return manifestResources
				}
				return normalizedUnique(meta.SupportsResources)
			}(),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (c *boundaryProviderCatalog) SupportedTriggers(providerID string) ([]string, error) {
	providerID = strings.TrimSpace(providerID)
	if providerID == "" {
		return nil, nil
	}
	list, err := c.ListProviders()
	if err != nil {
		return nil, err
	}
	for _, provider := range list {
		if provider.ID == providerID {
			return append([]string(nil), provider.SupportsTriggers...), nil
		}
	}
	return nil, nil
}

func (c *boundaryProviderCatalog) SupportsTrigger(providerID, trigger string) (bool, error) {
	trigger = strings.ToLower(strings.TrimSpace(trigger))
	if trigger == "" {
		return false, nil
	}
	triggers, err := c.SupportedTriggers(providerID)
	if err != nil {
		return false, err
	}
	for _, t := range triggers {
		if t == trigger {
			return true, nil
		}
	}
	return false, nil
}

func normalizedUnique(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, raw := range values {
		v := strings.ToLower(strings.TrimSpace(raw))
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

func appendUnique(values []string, value string) []string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return values
	}
	for _, v := range values {
		if strings.EqualFold(v, value) {
			return values
		}
	}
	values = append(values, value)
	sort.Strings(values)
	return values
}
