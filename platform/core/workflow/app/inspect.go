package app

import (
	"context"
	"os"

	providers "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"
	"github.com/runfabric/runfabric/platform/core/model/protocol"
	state "github.com/runfabric/runfabric/platform/core/state/core"
	"github.com/runfabric/runfabric/platform/core/state/transactions"
	"github.com/runfabric/runfabric/platform/state/locking"
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
	var workflowData any
	if receipt, err := ctx.Backends.Receipts.Load(ctx.Stage); err == nil {
		receiptData = receipt
		if runs, _ := state.ListWorkflowRuns(ctx.RootDir, ctx.Stage, 50); len(runs) > 0 {
			workflowData = map[string]any{
				"runs":        runs,
				"costSummary": state.WorkflowCostFromRuns(runs),
			}
		}
	} else if !os.IsNotExist(err) {
		receiptData = map[string]string{"error": err.Error()}
	}

	if provider, err := resolveProvider(ctx); err == nil {
		if orchestration, ok := provider.provider.(providers.OrchestrationCapable); ok {
			if odata, err := orchestration.InspectOrchestrations(context.Background(), providers.OrchestrationInspectRequest{Config: ctx.Config, Stage: ctx.Stage, Root: ctx.RootDir}); err == nil {
				if workflowData == nil {
					workflowData = map[string]any{}
				}
				if wm, ok := workflowData.(map[string]any); ok {
					if receipt, ok := receiptData.(*state.Receipt); ok && receipt.Metadata != nil {
						if len(receipt.Metadata) > 0 {
							odata["receiptMetadata"] = receipt.Metadata
						}
					}
					wm["orchestration"] = odata
				}
			}
		}
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
		Workflow: workflowData,
	}, nil
}
