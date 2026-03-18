package app

import (
	"github.com/runfabric/runfabric/engine/internal/config"
	"github.com/runfabric/runfabric/engine/internal/state"
)

// DashboardData is the data passed to the dashboard UI (project, stage, deploy status, optional AI workflow).
type DashboardData struct {
	Service       string
	Stage         string
	App           string // optional grouping (config.app)
	Org           string // optional org/tenant (config.org)
	Stages        []state.ReleaseEntry
	Receipt       *state.Receipt
	HasDeployment bool
	// AI Workflow (Phase 14): when aiWorkflow.enable is true
	AiWorkflowHash  string
	AiWorkflowEntry string
	AiWorkflowCost  *state.WorkflowCostSummary
}

// Dashboard loads config and receipt for the given stage and returns data for the dashboard UI.
func Dashboard(configPath, stage string) (*DashboardData, error) {
	ctx, err := Bootstrap(configPath, stage, "")
	if err != nil {
		return nil, err
	}
	data := &DashboardData{
		Service: ctx.Config.Service,
		Stage:   ctx.Stage,
		App:     ctx.Config.App,
		Org:     ctx.Config.Org,
	}
	releases, _ := ctx.Backends.Receipts.ListReleases()
	data.Stages = releases
	receipt, err := ctx.Backends.Receipts.Load(ctx.Stage)
	if err == nil && receipt != nil {
		data.Receipt = receipt
		data.HasDeployment = true
		if receipt.Metadata != nil {
			data.AiWorkflowHash = receipt.Metadata["aiWorkflowHash"]
			data.AiWorkflowEntry = receipt.Metadata["aiWorkflowEntrypoint"]
		}
	}
	if ctx.Config.AiWorkflow != nil && ctx.Config.AiWorkflow.Enable {
		if data.AiWorkflowHash == "" {
			if g, _ := config.CompileAiWorkflow(ctx.Config.AiWorkflow); g != nil {
				data.AiWorkflowHash = g.Hash
				data.AiWorkflowEntry = g.Entrypoint
			}
		}
		runs, _ := state.ListWorkflowRuns(ctx.RootDir, ctx.Stage, 20)
		if len(runs) > 0 {
			s := state.WorkflowCostFromRuns(runs)
			data.AiWorkflowCost = &s
		}
	}
	return data, nil
}
