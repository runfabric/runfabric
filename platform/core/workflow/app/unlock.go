package app

import (
	"fmt"
	"os"

	"github.com/runfabric/runfabric/platform/state/locking"
)

func Unlock(configPath, stage string, force bool) (any, error) {
	ctx, err := Bootstrap(configPath, stage, "")
	if err != nil {
		return nil, err
	}

	lockBackend := locking.NewFileBackend(ctx.RootDir)

	lockRecord, err := lockBackend.Read(ctx.Config.Service, ctx.Stage)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{
				"unlocked": false,
				"message":  "no lock found",
			}, nil
		}
		return nil, err
	}

	if !force {
		return nil, fmt.Errorf("lock exists for operation=%s pid=%d createdAt=%s; use --force to unlock",
			lockRecord.Operation, lockRecord.PID, lockRecord.CreatedAt)
	}

	if err := lockBackend.Release(ctx.Config.Service, ctx.Stage); err != nil {
		return nil, err
	}

	return map[string]any{
		"unlocked": true,
		"service":  ctx.Config.Service,
		"stage":    ctx.Stage,
	}, nil
}
