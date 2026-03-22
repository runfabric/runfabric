package app

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	appErrs "github.com/runfabric/runfabric/platform/core/model/errors"
	"github.com/runfabric/runfabric/platform/core/state/locking"
	"github.com/runfabric/runfabric/platform/core/state/transactions"
	"github.com/runfabric/runfabric/platform/core/workflow/recovery"
	awsprovider "github.com/runfabric/runfabric/platform/extensions/interfaces/providers/aws"
	azureprovider "github.com/runfabric/runfabric/platform/extensions/interfaces/providers/azure"
	gcpprovider "github.com/runfabric/runfabric/platform/extensions/interfaces/providers/gcp"
)

func Recover(configPath, stage string, mode recovery.Mode) (any, error) {
	if mode != recovery.ModeRollback && mode != recovery.ModeResume && mode != recovery.ModeInspect {
		return nil, appErrs.Wrap(appErrs.CodeDeployFailed, fmt.Sprintf("unsupported recovery mode %q (use rollback|resume|inspect)", mode), nil)
	}

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

	reg := recovery.NewRegistry()
	reg.Register("aws-lambda", func(j any) recovery.Handler { return awsprovider.NewRecoveryHandler(journal) })
	reg.Register("gcp-functions", func(j any) recovery.Handler { return gcpprovider.NewRecoveryHandler(journal) })
	reg.Register("azure-functions", func(j any) recovery.Handler { return azureprovider.NewRecoveryHandler(journal) })

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

	switch mode {
	case recovery.ModeResume:
		if ctx.Config.Provider.Name == "aws-lambda" {
			return awsprovider.ResumeDeploy(context.Background(), ctx.Config, ctx.Stage, ctx.RootDir, journal)
		}
		res, runErr := handler.Resume(context.Background(), req)
		if runErr != nil {
			return nil, runErr
		}
		if err := validateRecoveryResult(mode, res); err != nil {
			return nil, err
		}
		return res, nil
	case recovery.ModeRollback:
		res, runErr := handler.Rollback(context.Background(), req)
		if runErr != nil {
			return nil, runErr
		}
		if err := validateRecoveryResult(mode, res); err != nil {
			return nil, err
		}
		return res, nil
	case recovery.ModeInspect:
		res, runErr := handler.Inspect(context.Background(), req)
		if runErr != nil {
			return nil, runErr
		}
		if err := validateRecoveryResult(mode, res); err != nil {
			return nil, err
		}
		return res, nil
	default:
		return nil, appErrs.Wrap(appErrs.CodeDeployFailed, "unsupported recovery mode", nil)
	}
}

func validateRecoveryResult(mode recovery.Mode, res *recovery.Result) error {
	if res == nil {
		return appErrs.Wrap(appErrs.CodeDeployFailed, fmt.Sprintf("recovery %q returned no result", mode), nil)
	}
	status := strings.ToLower(strings.TrimSpace(res.Status))
	if status == "unsupported" || status == "not_implemented" || status == "" {
		return appErrs.Wrap(appErrs.CodeDeployFailed, fmt.Sprintf("recovery %q failed: unsupported result status", mode), nil)
	}
	return nil
}
