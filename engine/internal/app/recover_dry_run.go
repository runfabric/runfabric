package app

import (
	"os"

	"github.com/runfabric/runfabric/engine/internal/transactions"
)

func RecoverDryRun(configPath, stage string) (any, error) {
	ctx, err := Bootstrap(configPath, stage, "")
	if err != nil {
		return nil, err
	}

	journalBackend := transactions.NewFileBackend(ctx.RootDir)
	journal, err := journalBackend.Load(ctx.Config.Service, ctx.Stage)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{
				"canRecover": false,
				"message":    "no journal found",
			}, nil
		}
		return nil, err
	}

	ok, checksumErr := transactions.VerifyChecksum(journal)

	return map[string]any{
		"canRecover":    true,
		"service":       journal.Service,
		"stage":         journal.Stage,
		"status":        journal.Status,
		"attemptCount":  journal.AttemptCount,
		"checkpoints":   journal.Checkpoints,
		"operations":    len(journal.Operations),
		"checksumValid": ok,
		"checksumError": ErrString(checksumErr),
	}, nil
}

func ErrString(err error) string {
	if err == nil {
		return "ok"
	}
	if os.IsNotExist(err) {
		return "not found"
	}
	return err.Error()
}
