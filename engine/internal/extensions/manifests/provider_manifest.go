// Package manifests holds provider and addon manifest types and registries for RunFabric Extensions.
package manifests

import (
	"sort"
	"strings"
	"sync"
)

// PluginKind is the type of a RunFabric plugin (provider, runtime, or simulator).
type PluginKind string

const (
	KindProvider  PluginKind = "provider"
	KindRuntime   PluginKind = "runtime"
	KindSimulator PluginKind = "simulator"
)

// Permissions describes what a plugin or addon is allowed to access (for validation and UX).
type Permissions struct {
	FS      bool `json:"fs,omitempty"`
	Env     bool `json:"env,omitempty"`
	Network bool `json:"network,omitempty"`
	Cloud   bool `json:"cloud,omitempty"`
}

// PluginManifest describes a RunFabric Plugin (provider, runtime, simulator) for list/info/search.
type PluginManifest struct {
	ID          string      `json:"id"`
	Kind        PluginKind  `json:"kind"`
	Name        string      `json:"name,omitempty"`
	Description string      `json:"description,omitempty"`
	Permissions Permissions `json:"permissions,omitempty"`

	// Optional metadata for external plugins (Phase 15b). Built-ins omit these fields.
	Source     string `json:"source,omitempty"`     // builtin | external
	Version    string `json:"version,omitempty"`    // external plugin version (directory/manifest)
	Path       string `json:"path,omitempty"`       // absolute path to plugin directory
	Executable string `json:"executable,omitempty"` // resolved executable path (optional)
}

// PluginRegistry holds metadata for built-in and (future) external plugins.
type PluginRegistry struct {
	mu      sync.RWMutex
	plugins map[string]*PluginManifest

	// lowercased fields for Search() without per-call allocations
	idLower   map[string]string
	nameLower map[string]string

	// cached sorted lists for List() (invalidate on Register())
	cacheDirty  bool
	cacheAll    []*PluginManifest
	cacheByKind map[PluginKind][]*PluginManifest
}

// NewPluginRegistry returns a registry pre-filled with built-in provider and runtime manifests.
func NewPluginRegistry() *PluginRegistry {
	r := &PluginRegistry{
		plugins:     make(map[string]*PluginManifest),
		idLower:     make(map[string]string),
		nameLower:   make(map[string]string),
		cacheDirty:  true,
		cacheByKind: make(map[PluginKind][]*PluginManifest),
	}
	for _, m := range builtinPluginManifests() {
		r.plugins[m.ID] = m
		r.idLower[m.ID] = strings.ToLower(m.ID)
		r.nameLower[m.ID] = strings.ToLower(m.Name)
	}
	return r
}

func builtinPluginManifests() []*PluginManifest {
	return []*PluginManifest{
		// Providers (deploy targets)
		{ID: "aws", Kind: KindProvider, Name: "AWS Lambda", Description: "AWS Lambda (legacy alias for aws-lambda)"},
		{ID: "aws-lambda", Kind: KindProvider, Name: "AWS Lambda", Description: "Deploy and run functions on AWS Lambda"},
		{ID: "gcp-functions", Kind: KindProvider, Name: "GCP Cloud Functions", Description: "Deploy and run functions on GCP Cloud Functions Gen 2"},
		{ID: "azure-functions", Kind: KindProvider, Name: "Azure Functions", Description: "Deploy and run functions on Azure Functions"},
		{ID: "alibaba-fc", Kind: KindProvider, Name: "Alibaba FC", Description: "Deploy and run functions on Alibaba Cloud Function Compute"},
		{ID: "cloudflare-workers", Kind: KindProvider, Name: "Cloudflare Workers", Description: "Deploy and run functions on Cloudflare Workers"},
		{ID: "digitalocean-functions", Kind: KindProvider, Name: "DigitalOcean Functions", Description: "Deploy and run functions on DigitalOcean App Platform"},
		{ID: "fly-machines", Kind: KindProvider, Name: "Fly.io Machines", Description: "Deploy and run functions on Fly.io Machines"},
		{ID: "ibm-openwhisk", Kind: KindProvider, Name: "IBM OpenWhisk", Description: "Deploy and run functions on IBM Cloud Functions (OpenWhisk)"},
		{ID: "kubernetes", Kind: KindProvider, Name: "Kubernetes", Description: "Deploy and run functions on Kubernetes"},
		{ID: "netlify", Kind: KindProvider, Name: "Netlify", Description: "Deploy and run functions on Netlify Functions"},
		{ID: "vercel", Kind: KindProvider, Name: "Vercel", Description: "Deploy and run functions on Vercel Serverless"},
		// Runtimes
		{ID: "nodejs", Kind: KindRuntime, Name: "Node.js", Description: "Node.js runtime (build and invoke)"},
		{ID: "runtime-node", Kind: KindRuntime, Name: "Node.js", Description: "Node.js runtime (alias)"},
		{ID: "python", Kind: KindRuntime, Name: "Python", Description: "Python runtime (build and invoke)"},
		{ID: "runtime-python", Kind: KindRuntime, Name: "Python", Description: "Python runtime (alias)"},
	}
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
	r.cacheDirty = true
}

// Get returns the plugin manifest for id, or nil.
func (r *PluginRegistry) Get(id string) *PluginManifest {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.plugins[id]
}

// List returns all plugins, optionally filtered by kind, sorted by kind (provider, runtime, simulator) then by ID.
func (r *PluginRegistry) List(kind PluginKind) []*PluginManifest {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rebuildCachesLocked()
	if kind == "" {
		return append([]*PluginManifest(nil), r.cacheAll...)
	}
	return append([]*PluginManifest(nil), r.cacheByKind[kind]...)
}

func kindOrder(k PluginKind) int {
	switch k {
	case KindProvider:
		return 0
	case KindRuntime:
		return 1
	case KindSimulator:
		return 2
	default:
		return 3
	}
}

// Search returns plugins whose id or name contains the query (case-insensitive).
func (r *PluginRegistry) Search(query string) []*PluginManifest {
	if query == "" {
		return r.List("")
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	q := strings.ToLower(query)
	var out []*PluginManifest
	for _, m := range r.plugins {
		id := r.idLower[m.ID]
		name := r.nameLower[m.ID]
		if strings.Contains(id, q) || strings.Contains(name, q) {
			out = append(out, m)
		}
	}
	return out
}

func (r *PluginRegistry) rebuildCachesLocked() {
	if !r.cacheDirty {
		return
	}
	all := make([]*PluginManifest, 0, len(r.plugins))
	byKind := make(map[PluginKind][]*PluginManifest, 3)
	for _, m := range r.plugins {
		all = append(all, m)
		byKind[m.Kind] = append(byKind[m.Kind], m)
	}
	sort.Slice(all, func(i, j int) bool {
		ki, kj := all[i].Kind, all[j].Kind
		if ki != kj {
			return kindOrder(ki) < kindOrder(kj)
		}
		return all[i].ID < all[j].ID
	})
	for k, slice := range byKind {
		sort.Slice(slice, func(i, j int) bool { return slice[i].ID < slice[j].ID })
		byKind[k] = slice
	}
	r.cacheAll = all
	r.cacheByKind = byKind
	r.cacheDirty = false
}
