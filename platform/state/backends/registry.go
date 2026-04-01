package backends

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

type Bundle struct {
	Locks    LockBackend
	Journals JournalBackend
	Receipts ReceiptBackend
}

type BundleFactory func(ctx context.Context, opts Options) (*Bundle, error)

var (
	bundleFactoriesMu sync.RWMutex
	bundleFactories   = map[string]BundleFactory{}
)

// RegisterBundleFactory registers a backend bundle constructor for a kind.
// Kind matching is case-insensitive.
func RegisterBundleFactory(kind string, factory BundleFactory) {
	kind = normalizeBundleKind(kind)
	if kind == "" || factory == nil {
		return
	}
	bundleFactoriesMu.Lock()
	bundleFactories[kind] = factory
	bundleFactoriesMu.Unlock()
}

func bundleFactoryFor(kind string) BundleFactory {
	bundleFactoriesMu.RLock()
	defer bundleFactoriesMu.RUnlock()
	return bundleFactories[kind]
}

func normalizeBundleKind(kind string) string {
	kind = strings.ToLower(strings.TrimSpace(kind))
	if kind == "" {
		return "local"
	}
	return kind
}

func NewBundle(ctx context.Context, opts Options) (*Bundle, error) {
	kind := normalizeBundleKind(opts.Kind)
	factory := bundleFactoryFor(kind)
	if factory == nil {
		return nil, fmt.Errorf("unsupported backend kind: %s", opts.Kind)
	}
	return factory(ctx, opts)
}
