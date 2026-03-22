package app

import (
	"fmt"
	"time"

	"github.com/runfabric/runfabric/platform/core/state/locking"
)

func LockSteal(configPath, stage string) (any, error) {
	ctx, err := Bootstrap(configPath, stage, "")
	if err != nil {
		return nil, err
	}

	lb, ok := ctx.Backends.Locks.(interface {
		Steal(service, stage, newOperation string, staleAfter time.Duration) (*locking.Handle, error)
		Kind() string
	})
	if !ok {
		return nil, fmt.Errorf("backend %s does not support lock stealing", ctx.Backends.Locks.Kind())
	}

	handle, err := lb.Steal(ctx.Config.Service, ctx.Stage, "steal", 15*time.Minute)
	if err != nil {
		return nil, err
	}
	defer func() { _ = handle.Release() }()

	return map[string]any{
		"stolen":     true,
		"service":    ctx.Config.Service,
		"stage":      ctx.Stage,
		"ownerToken": handle.OwnerToken,
		"backend":    lb.Kind(),
	}, nil
}
