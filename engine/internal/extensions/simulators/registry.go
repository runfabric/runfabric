package simulators

import (
	"fmt"
	"strings"
	"sync"
)

type Registry struct {
	mu         sync.RWMutex
	simulators map[string]Simulator
}

func NewRegistry() *Registry {
	return &Registry{simulators: map[string]Simulator{}}
}

func (r *Registry) Register(sim Simulator) error {
	if sim == nil {
		return fmt.Errorf("simulator plugin is nil")
	}
	id := strings.TrimSpace(sim.Meta().ID)
	if id == "" {
		return fmt.Errorf("simulator plugin id is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.simulators[id] = sim
	return nil
}

func (r *Registry) Get(id string) (Simulator, error) {
	id = strings.TrimSpace(id)
	r.mu.RLock()
	defer r.mu.RUnlock()
	sim, ok := r.simulators[id]
	if !ok {
		return nil, fmt.Errorf("simulator plugin %q is not registered", id)
	}
	return sim, nil
}

func (r *Registry) List() []Meta {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Meta, 0, len(r.simulators))
	for _, sim := range r.simulators {
		out = append(out, sim.Meta())
	}
	return out
}
