package app

import (
	"os"

	"github.com/runfabric/runfabric/engine/internal/locking"
	"github.com/runfabric/runfabric/engine/internal/state"
	"github.com/runfabric/runfabric/engine/internal/transactions"
	"github.com/runfabric/runfabric/engine/pkg/protocol"
)

func Inspect(configPath, stage string) (any, error) {
	ctx, err := Bootstrap(configPath, stage, "")
	if err != nil {
		return nil, err
	}

	lockBackend := locking.NewFileBackend(ctx.RootDir)
	journalBackend := transactions.NewFileBackend(ctx.RootDir)

	var lockData any
	if lock, err := lockBackend.Read(ctx.Config.Service, ctx.Stage); err == nil {
		lockData = lock
	} else if !os.IsNotExist(err) {
		lockData = map[string]string{"error": err.Error()}
	}

	var journalData any
	if journal, err := journalBackend.Load(ctx.Config.Service, ctx.Stage); err == nil {
		journalData = journal
	} else if !os.IsNotExist(err) {
		journalData = map[string]string{"error": err.Error()}
	}

	var receiptData any
	var aiWorkflowData any
	if receipt, err := ctx.Backends.Receipts.Load(ctx.Stage); err == nil {
		receiptData = receipt
		if receipt.Metadata != nil {
			if h := receipt.Metadata["aiWorkflowHash"]; h != "" {
				entry := receipt.Metadata["aiWorkflowEntrypoint"]
				aiWorkflowData = map[string]any{
					"hash":       h,
					"entrypoint": entry,
				}
				if runs, _ := state.ListWorkflowRuns(ctx.RootDir, ctx.Stage, 50); len(runs) > 0 {
					summary := state.WorkflowCostFromRuns(runs)
					aiWorkflowData.(map[string]any)["costSummary"] = summary
				}
			}
		}
	} else if !os.IsNotExist(err) {
		receiptData = map[string]string{"error": err.Error()}
	}

	return &protocol.InspectResult{
		Service: ctx.Config.Service,
		Stage:   ctx.Stage,
		Lock: map[string]any{
			"backend": ctx.Backends.Locks.Kind(),
			"record":  lockData,
		},
		Journal: map[string]any{
			"backend": ctx.Backends.Journals.Kind(),
			"record":  journalData,
		},
		Receipt: map[string]any{
			"backend": ctx.Backends.Receipts.Kind(),
			"record":  receiptData,
		},
		AiWorkflow: aiWorkflowData,
	}, nil
}
