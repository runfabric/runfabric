package providers

import (
	"sort"
	"sync"
)

// Registry holds provider plugins by name and implements ProviderRegistry.
// For backward compatibility, Get returns a Provider (adapter over the plugin);
// use List() for ProviderMeta. Register() accepts legacy Provider; RegisterPlugin() accepts ProviderPlugin.
type Registry struct {
	mu      sync.RWMutex
	plugins map[string]ProviderPlugin
	// providers caches the legacy Provider wrapper for each plugin name, to avoid
	// allocating a new adapter on every Get().
	providers map[string]Provider
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{
		plugins:   make(map[string]ProviderPlugin),
		providers: make(map[string]Provider),
	}
}

// Register adds a legacy Provider by wrapping it as ProviderPlugin and storing under Name().
func (r *Registry) Register(p Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	name := p.Name()
	r.plugins[name] = &legacyAdapter{Provider: p}
	// For legacy providers, return the provider directly (no wrapper).
	r.providers[name] = p
}

// RegisterPlugin adds a ProviderPlugin under Meta().Name. Implements ProviderRegistry.
func (r *Registry) RegisterPlugin(p ProviderPlugin) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	name := p.Meta().Name
	r.plugins[name] = p
	r.providers[name] = &providerFromPlugin{ProviderPlugin: p}
	return nil
}

// Get returns the provider for name as the legacy Provider interface (adapter over the plugin).
func (r *Registry) Get(name string) (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	prov, ok := r.providers[name]
	if !ok {
		return nil, ErrProviderNotFound(name)
	}
	return prov, nil
}

// GetPlugin returns the ProviderPlugin for name, if present. Use when calling the new interface directly.
func (r *Registry) GetPlugin(name string) (ProviderPlugin, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.plugins[name]
	return p, ok
}

// List returns metadata for all registered plugins. Implements ProviderRegistry.
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
