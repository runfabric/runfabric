package runtimes

import (
	"fmt"
	"strings"
	"sync"
)

// Registry holds runtime plugins by ID.
type Registry struct {
	mu       sync.RWMutex
	runtimes map[string]Runtime
}

func NewRegistry() *Registry {
	return &Registry{runtimes: map[string]Runtime{}}
}

func (r *Registry) Register(rt Runtime) error {
	if rt == nil {
		return fmt.Errorf("runtime plugin is nil")
	}
	id := strings.TrimSpace(rt.Meta().ID)
	if id == "" {
		return fmt.Errorf("runtime plugin id is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.runtimes[id] = rt
	return nil
}

func (r *Registry) Get(runtimeID string) (Runtime, error) {
	id := NormalizeRuntimeID(runtimeID)
	r.mu.RLock()
	defer r.mu.RUnlock()
	rt, ok := r.runtimes[id]
	if !ok {
		return nil, fmt.Errorf("runtime plugin %q is not registered", strings.TrimSpace(runtimeID))
	}
	return rt, nil
}

func (r *Registry) List() []Meta {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Meta, 0, len(r.runtimes))
	for _, rt := range r.runtimes {
		out = append(out, rt.Meta())
	}
	return out
}

// NormalizeRuntimeID maps versioned runtime values to runtime plugin IDs.
func NormalizeRuntimeID(runtime string) string {
	raw := strings.ToLower(strings.TrimSpace(runtime))
	switch {
	case raw == "runtime-node":
		return "nodejs"
	case strings.HasPrefix(raw, "nodejs"):
		return "nodejs"
	case raw == "runtime-python":
		return "python"
	case strings.HasPrefix(raw, "python"):
		return "python"
	default:
		return strings.TrimSpace(runtime)
	}
}
