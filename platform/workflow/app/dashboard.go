package app

import (
	state "github.com/runfabric/runfabric/platform/core/state/core"
)

// DashboardData is the data passed to the dashboard UI (project, stage, deploy status, workflows).
type DashboardData struct {
	Service          string
	Stage            string
	App              string // optional grouping (config.app)
	Org              string // optional org/tenant (config.org)
	Stages           []state.ReleaseEntry
	Receipt          *state.Receipt
	HasDeployment    bool
	WorkflowRunCount int
	WorkflowCost     *state.WorkflowCostSummary
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
	}
	runs, _ := state.ListWorkflowRuns(ctx.RootDir, ctx.Stage, 20)
	data.WorkflowRunCount = len(runs)
	if len(runs) > 0 {
		s := state.WorkflowCostFromRuns(runs)
		data.WorkflowCost = &s
	}
	return data, nil
}
