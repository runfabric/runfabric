package provisioning

import (
	"context"
	"errors"
	"sync"
)

// ErrProvisioningUnsupported is returned when resource provisioning cannot be resolved from provider metadata.
var ErrProvisioningUnsupported = errors.New("resource provisioning is unavailable for this configuration; use connectionStringEnv or connectionString")

// ErrNotImplemented is kept as a compatibility alias.
var ErrNotImplemented = ErrProvisioningUnsupported

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
