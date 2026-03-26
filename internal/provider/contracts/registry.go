package contracts

import (
	"sort"
	"sync"
)

// Registry holds provider plugins by name and implements ProviderRegistry.
type Registry struct {
	mu      sync.RWMutex
	plugins map[string]ProviderPlugin
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{plugins: make(map[string]ProviderPlugin)}
}

// Register adds a provider plugin under Meta().Name.
func (r *Registry) Register(p ProviderPlugin) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.plugins[p.Meta().Name] = p
	return nil
}

// Get returns the provider plugin for name.
func (r *Registry) Get(name string) (ProviderPlugin, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.plugins[name]
	return p, ok
}

// List returns metadata for all registered plugins.
func (r *Registry) List() []ProviderMeta {
	r.mu.RLock()
	out := make([]ProviderMeta, 0, len(r.plugins))
	for _, p := range r.plugins {
		out = append(out, p.Meta())
	}
	r.mu.RUnlock()
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
