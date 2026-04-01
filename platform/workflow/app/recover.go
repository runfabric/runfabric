package app

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	providers "github.com/runfabric/runfabric/internal/provider/contracts"
	appErrs "github.com/runfabric/runfabric/platform/core/model/errors"
	"github.com/runfabric/runfabric/platform/core/state/transactions"
	"github.com/runfabric/runfabric/platform/state/locking"
	"github.com/runfabric/runfabric/platform/workflow/recovery"
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

	provider, err := resolveProvider(ctx)
	if err != nil {
		return nil, err
	}
	recoveryProvider, ok := provider.provider.(providers.RecoveryCapable)
	if !ok {
		return nil, appErrs.Wrap(appErrs.CodeDeployFailed, fmt.Sprintf("provider %q does not support recovery", provider.name), nil)
	}

	req := providers.RecoveryRequest{
		Config:  ctx.Config,
		Root:    ctx.RootDir,
		Service: ctx.Config.Service,
		Stage:   ctx.Stage,
		Region:  ctx.Config.Provider.Region,
		Mode:    string(mode),
		Journal: journal,
	}
	res, runErr := recoveryProvider.Recover(context.Background(), req)
	if runErr != nil {
		return nil, runErr
	}
	if mode == recovery.ModeResume && len(res.ResumeData) > 0 {
		return res.ResumeData, nil
	}
	out := &recovery.Result{
		Recovered: res.Recovered,
		Mode:      res.Mode,
		Status:    res.Status,
		Message:   res.Message,
		Metadata:  res.Metadata,
		Errors:    res.Errors,
	}
	if err := validateRecoveryResult(mode, out); err != nil {
		return nil, err
	}
	return out, nil
}

// RecoverByMode parses a textual recovery mode (rollback|resume|inspect) and delegates to Recover.
func RecoverByMode(configPath, stage, mode string) (any, error) {
	return Recover(configPath, stage, recovery.Mode(strings.ToLower(strings.TrimSpace(mode))))
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
