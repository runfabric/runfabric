package app

import (
	"context"
	"os"
	"time"

	appErrs "github.com/runfabric/runfabric/engine/internal/errors"
	"github.com/runfabric/runfabric/engine/internal/locking"
	"github.com/runfabric/runfabric/engine/internal/recovery"
	"github.com/runfabric/runfabric/engine/internal/transactions"
	awsprovider "github.com/runfabric/runfabric/engine/providers/aws"
)

func Recover(configPath, stage string, mode recovery.Mode) (any, error) {
	ctx, err := Bootstrap(configPath, stage, "")
	if err != nil {
		return nil, err
	}

	journalBackend := transactions.NewFileBackend(ctx.RootDir)
	journal, err := journalBackend.Load(ctx.Config.Service, ctx.Stage)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{
				"recovered": false,
				"message":   "no unfinished journal found",
			}, nil
		}
		return nil, appErrs.Wrap(appErrs.CodeDeployFailed, "load recovery journal failed", err)
	}

	if journal.AttemptCount >= 3 && mode == recovery.ModeRollback {
		archivePath, archErr := recovery.ArchiveJournal(ctx.RootDir, journal)
		if archErr != nil {
			return nil, archErr
		}
		return map[string]any{
			"recovered": false,
			"status":    "archived",
			"message":   "journal archived after repeated failed attempts",
			"archive":   archivePath,
		}, nil
	}

	lockBackend := locking.NewFileBackend(ctx.RootDir)
	lockHandle, err := lockBackend.Acquire(ctx.Config.Service, ctx.Stage, "recover", time.Minute)
	if err != nil {
		return nil, appErrs.Wrap(appErrs.CodeDeployFailed, "acquire recovery lock failed", err)
	}
	defer func() { _ = lockHandle.Release() }()

	switch mode {
	case recovery.ModeResume:
		return awsprovider.ResumeDeploy(context.Background(), ctx.Config, ctx.Stage, ctx.RootDir, journal)
	case recovery.ModeRollback, recovery.ModeInspect:
		reg := recovery.NewRegistry()
		reg.Register("aws", func(j any) recovery.Handler {
			return awsprovider.NewRecoveryHandler(journal)
		})

		handler, err := reg.Get(ctx.Config.Provider.Name, journal)
		if err != nil {
			return nil, err
		}

		req := recovery.Request{
			Root:    ctx.RootDir,
			Service: ctx.Config.Service,
			Stage:   ctx.Stage,
			Region:  ctx.Config.Provider.Region,
			Mode:    mode,
		}

		if mode == recovery.ModeRollback {
			return handler.Rollback(context.Background(), req)
		}
		return handler.Inspect(context.Background(), req)

	default:
		return nil, appErrs.Wrap(appErrs.CodeDeployFailed, "unsupported recovery mode", nil)
	}
}
