package providers

import (
	"sort"
	"strings"
)

// CapabilitySet provides fast, case-insensitive capability checks for provider metadata.
type CapabilitySet struct {
	all   []string
	index map[string]struct{}
}

// NewCapabilitySet builds a normalized capability set from provider metadata.
func NewCapabilitySet(meta ProviderMeta) CapabilitySet {
	index := map[string]struct{}{}
	all := make([]string, 0, len(meta.Capabilities))
	for _, raw := range meta.Capabilities {
		name := strings.ToLower(strings.TrimSpace(raw))
		if name == "" {
			continue
		}
		if _, ok := index[name]; ok {
			continue
		}
		index[name] = struct{}{}
		all = append(all, name)
	}
	sort.Strings(all)
	return CapabilitySet{all: all, index: index}
}

// List returns normalized capability names in deterministic order.
func (c CapabilitySet) List() []string {
	return append([]string(nil), c.all...)
}

// Has reports whether capability exists.
func (c CapabilitySet) Has(capability string) bool {
	_, ok := c.index[strings.ToLower(strings.TrimSpace(capability))]
	return ok
}

// HasAny reports whether any of the provided capabilities are present.
func (c CapabilitySet) HasAny(capabilities ...string) bool {
	for _, capability := range capabilities {
		if c.Has(capability) {
			return true
		}
	}
	return false
}
