package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/runfabric/runfabric/engine/internal/config"
	deployapi "github.com/runfabric/runfabric/engine/internal/deploy/api"
	"github.com/runfabric/runfabric/engine/internal/lifecycle"
	"github.com/runfabric/runfabric/engine/internal/state"
)

func Invoke(configPath, stage, function, providerOverride string, payload []byte) (any, error) {
	ctx, err := Bootstrap(configPath, stage, providerOverride)
	if err != nil {
		return nil, err
	}
	provider := ctx.Config.Provider.Name
	start := time.Now().UTC()

	var workflowHash, entrypoint, nodeID, nodeType string
	if ctx.Config.AiWorkflow != nil && ctx.Config.AiWorkflow.Enable {
		if g, compileErr := config.CompileAiWorkflow(ctx.Config.AiWorkflow); compileErr == nil && g != nil {
			workflowHash = g.Hash
			entrypoint = g.Entrypoint
			// Best-effort mapping: node.params.function == invoked function
			for _, n := range ctx.Config.AiWorkflow.Nodes {
				if n.Params == nil {
					continue
				}
				if fn, ok := n.Params["function"].(string); ok && fn == function {
					nodeID = n.ID
					nodeType = n.Type
					break
				}
			}
		}
	}

	// Helper to persist a minimal run record (best-effort; never blocks invoke success).
	saveRun := func(status state.WorkflowRunStatus, nodeStatus state.NodeRunStatus, errMsg string) string {
		if workflowHash == "" {
			return ""
		}
		runID := newRunID()
		end := time.Now().UTC()
		run := &state.WorkflowRun{
			RunID:        runID,
			Service:      ctx.Config.Service,
			Stage:        ctx.Stage,
			Provider:     provider,
			WorkflowHash: workflowHash,
			Entrypoint:   entrypoint,
			Status:       status,
			StartedAt:    start.Format(time.RFC3339),
			EndedAt:      end.Format(time.RFC3339),
			DurationMs:   end.Sub(start).Milliseconds(),
		}
		if nodeID != "" {
			run.Nodes = []state.NodeRun{{
				NodeID:     nodeID,
				NodeType:   nodeType,
				Status:     nodeStatus,
				StartedAt:  start.Format(time.RFC3339),
				EndedAt:    end.Format(time.RFC3339),
				DurationMs: end.Sub(start).Milliseconds(),
				Error:      errMsg,
			}}
		}
		_ = state.SaveWorkflowRun(ctx.RootDir, run)
		return runID
	}

	if deployapi.HasInvoker(provider) {
		res, err := deployapi.Invoke(context.Background(), provider, ctx.Config, ctx.Stage, function, payload, ctx.RootDir)
		if err != nil {
			_ = saveRun(state.RunStatusFailed, state.NodeStatusFailed, err.Error())
			return nil, err
		}
		runID := saveRun(state.RunStatusOK, state.NodeStatusOK, "")
		if runID != "" {
			res.RunID = runID
			res.Workflow = workflowHash
		}
		return res, nil
	}
	res, err := lifecycle.Invoke(ctx.Registry, ctx.Config, ctx.Stage, function, payload)
	if err != nil {
		_ = saveRun(state.RunStatusFailed, state.NodeStatusFailed, err.Error())
		return nil, err
	}
	runID := saveRun(state.RunStatusOK, state.NodeStatusOK, "")
	if runID != "" {
		res.RunID = runID
		res.Workflow = workflowHash
	}
	return res, nil
}

func newRunID() string {
	var b [12]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
