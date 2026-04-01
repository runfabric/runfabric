// Package manifests holds provider and addon manifest types and registries for RunFabric Extensions.
package manifests

import (
	"sort"
	"strings"
	"sync"

	"github.com/runfabric/runfabric/platform/extensions/providerpolicy"
)

// PluginKind is the type of a RunFabric plugin (provider, runtime, simulator, router, secret-manager, or state).
type PluginKind string

const (
	KindProvider      PluginKind = "provider"
	KindRuntime       PluginKind = "runtime"
	KindSimulator     PluginKind = "simulator"
	KindRouter        PluginKind = "router"
	KindSecretManager PluginKind = "secret-manager"
	KindState         PluginKind = "state"
)

// NormalizePluginKind normalizes plugin kind input to singular names.
func NormalizePluginKind(raw string) PluginKind {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "provider":
		return KindProvider
	case "runtime":
		return KindRuntime
	case "simulator":
		return KindSimulator
	case "router":
		return KindRouter
	case "secret-manager":
		return KindSecretManager
	case "state":
		return KindState
	default:
		return PluginKind(strings.ToLower(strings.TrimSpace(raw)))
	}
}

// IsSupportedPluginKind reports whether kind is one of provider/runtime/simulator/router/secret-manager/state.
func IsSupportedPluginKind(kind PluginKind) bool {
	switch kind {
	case KindProvider, KindRuntime, KindSimulator, KindRouter, KindSecretManager, KindState:
		return true
	default:
		return false
	}
}

// Permissions describes what a plugin or addon is allowed to access (for validation and UX).
type Permissions struct {
	FS      bool `json:"fs,omitempty"`
	Env     bool `json:"env,omitempty"`
	Network bool `json:"network,omitempty"`
	Cloud   bool `json:"cloud,omitempty"`
}

// PluginManifest describes a RunFabric Plugin (provider, runtime, simulator, router, secret-manager, state) for list/info/search.
type PluginManifest struct {
	ID                string      `json:"id"`
	Kind              PluginKind  `json:"kind"`
	Name              string      `json:"name,omitempty"`
	Description       string      `json:"description,omitempty"`
	Permissions       Permissions `json:"permissions,omitempty"`
	Capabilities      []string    `json:"capabilities,omitempty"`
	SupportsRuntime   []string    `json:"supportsRuntime,omitempty"`
	SupportsTriggers  []string    `json:"supportsTriggers,omitempty"`
	SupportsResources []string    `json:"supportsResources,omitempty"`

	// Optional metadata for external plugins (Phase 15b). Built-ins omit these fields.
	Source     string `json:"source,omitempty"`     // builtin | external
	Version    string `json:"version,omitempty"`    // external plugin version (directory/manifest)
	Path       string `json:"path,omitempty"`       // absolute path to plugin directory
	Executable string `json:"executable,omitempty"` // resolved executable path (optional)
}

// PluginRegistry holds metadata for built-in and external plugins.
type PluginRegistry struct {
	mu      sync.RWMutex
	plugins map[string]*PluginManifest

	// lowercased fields for Search() without per-call allocations
	idLower   map[string]string
	nameLower map[string]string
}

// NewEmptyPluginRegistry returns an empty plugin registry without preloaded built-ins.
func NewEmptyPluginRegistry() *PluginRegistry {
	return &PluginRegistry{
		plugins:   make(map[string]*PluginManifest),
		idLower:   make(map[string]string),
		nameLower: make(map[string]string),
	}
}

// NewPluginRegistry returns a registry pre-filled with built-in provider and runtime manifests.
func NewPluginRegistry() *PluginRegistry {
	r := NewEmptyPluginRegistry()
	for _, m := range builtinPluginManifests() {
		r.plugins[m.ID] = m
		r.idLower[m.ID] = strings.ToLower(m.ID)
		r.nameLower[m.ID] = strings.ToLower(m.Name)
	}
	return r
}

func builtinPluginManifests() []*PluginManifest {
	list := make([]*PluginManifest, 0, 16)
	for _, p := range providerpolicy.BuiltinManifestProviders() {
		list = append(list, &PluginManifest{
			ID:          p.ID,
			Kind:        KindProvider,
			Name:        p.Name,
			Description: p.Description,
		})
	}
	return list
}

// Register adds or overwrites a plugin manifest.
func (r *PluginRegistry) Register(m *PluginManifest) {
	if m == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.plugins[m.ID] = m
	r.idLower[m.ID] = strings.ToLower(m.ID)
	r.nameLower[m.ID] = strings.ToLower(m.Name)
}

// Get returns the plugin manifest for id, or nil.
func (r *PluginRegistry) Get(id string) *PluginManifest {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.plugins[id]
}

// List returns all plugins, optionally filtered by kind, sorted by kind (provider, runtime, simulator, router, secret-manager, state) then by ID.
func (r *PluginRegistry) List(kind PluginKind) []*PluginManifest {
	r.mu.RLock()
	list := make([]*PluginManifest, 0, len(r.plugins))
	for _, m := range r.plugins {
		if kind == "" || m.Kind == kind {
			list = append(list, m)
		}
	}
	r.mu.RUnlock()
	sort.Slice(list, func(i, j int) bool {
		ki, kj := list[i].Kind, list[j].Kind
		if ki != kj {
			return kindOrder(ki) < kindOrder(kj)
		}
		return list[i].ID < list[j].ID
	})
	return list
}

func kindOrder(k PluginKind) int {
	switch k {
	case KindProvider:
		return 0
	case KindRuntime:
		return 1
	case KindSimulator:
		return 2
	case KindRouter:
		return 3
	case KindSecretManager:
		return 4
	case KindState:
		return 5
	default:
		return 6
	}
}

// Search returns plugins whose id or name contains the query (case-insensitive).
func (r *PluginRegistry) Search(query string) []*PluginManifest {
	if query == "" {
		return r.List("")
	}
	r.mu.RLock()
	q := strings.ToLower(query)
	var out []*PluginManifest
	for _, m := range r.plugins {
		id := r.idLower[m.ID]
		name := r.nameLower[m.ID]
		if strings.Contains(id, q) || strings.Contains(name, q) {
			out = append(out, m)
		}
	}
	r.mu.RUnlock()
	sort.Slice(out, func(i, j int) bool {
		ki, kj := out[i].Kind, out[j].Kind
		if ki != kj {
			return kindOrder(ki) < kindOrder(kj)
		}
		return out[i].ID < out[j].ID
	})
	return out
}
