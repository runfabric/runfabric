package routers

import (
	"fmt"
	"strings"
	"sync"

	sdkrouter "github.com/runfabric/runfabric/plugin-sdk/go/router"
)

type Registry struct {
	mu      sync.RWMutex
	routers map[string]sdkrouter.Router
}

func NewRegistry() *Registry {
	return &Registry{routers: map[string]sdkrouter.Router{}}
}

func (r *Registry) Register(router sdkrouter.Router) error {
	if router == nil {
		return fmt.Errorf("router plugin is nil")
	}
	id := strings.TrimSpace(router.Meta().ID)
	if id == "" {
		return fmt.Errorf("router plugin id is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.routers[id] = router
	return nil
}

func (r *Registry) Get(id string) (sdkrouter.Router, error) {
	id = strings.TrimSpace(strings.ToLower(id))
	r.mu.RLock()
	defer r.mu.RUnlock()
	router, ok := r.routers[id]
	if !ok {
		return nil, fmt.Errorf("router plugin %q is not registered", id)
	}
	return router, nil
}

func (r *Registry) List() []sdkrouter.PluginMeta {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]sdkrouter.PluginMeta, 0, len(r.routers))
	for _, rt := range r.routers {
		out = append(out, rt.Meta())
	}
	return out
}
