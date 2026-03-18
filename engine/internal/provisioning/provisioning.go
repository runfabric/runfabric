package provisioning

import (
	"context"
	"errors"
	"sync"
)

// ErrNotImplemented is returned when a provider does not support resource provisioning yet.
var ErrNotImplemented = errors.New("resource provisioning not implemented for this provider; use connectionStringEnv or connectionString")

// Provisioner provisions a managed resource (e.g. RDS, ElastiCache) and returns its connection string.
type Provisioner interface {
	Provision(ctx context.Context, provider, resourceKey string, spec map[string]any) (connectionString string, err error)
}

var (
	registry   = make(map[string]Provisioner)
	registryMu sync.RWMutex
)

// Register registers a provisioner for the given provider name (e.g. "aws", "gcp").
func Register(provider string, p Provisioner) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[provider] = p
}

// Get returns the provisioner for the given provider, or nil.
func Get(provider string) Provisioner {
	registryMu.RLock()
	defer registryMu.RUnlock()
	return registry[provider]
}
