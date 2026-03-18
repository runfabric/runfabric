package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// WorkflowRunStatus is the status for a workflow run.
type WorkflowRunStatus string

const (
	RunStatusRunning WorkflowRunStatus = "running"
	RunStatusOK      WorkflowRunStatus = "ok"
	RunStatusFailed  WorkflowRunStatus = "failed"
)

// NodeRunStatus is the status for a node run.
type NodeRunStatus string

const (
	NodeStatusRunning NodeRunStatus = "running"
	NodeStatusOK      NodeRunStatus = "ok"
	NodeStatusFailed  NodeRunStatus = "failed"
)

// NodeRun is one node execution record within a workflow run.
// Phase 14.6: InputTokens, OutputTokens, EstimatedCostUSD for per-node model usage and cost.
type NodeRun struct {
	NodeID           string        `json:"nodeId"`
	NodeType         string        `json:"nodeType,omitempty"`
	Status           NodeRunStatus `json:"status"`
	StartedAt        string        `json:"startedAt,omitempty"`
	EndedAt          string        `json:"endedAt,omitempty"`
	DurationMs       int64         `json:"durationMs,omitempty"`
	Error            string        `json:"error,omitempty"`
	InputTokens      int64         `json:"inputTokens,omitempty"`
	OutputTokens     int64         `json:"outputTokens,omitempty"`
	EstimatedCostUSD float64       `json:"estimatedCostUsd,omitempty"`
}

// WorkflowRun is one workflow run record.
// Phase 14.6: TotalInputTokens, TotalOutputTokens, EstimatedCostUSD aggregate per-run cost.
type WorkflowRun struct {
	RunID             string            `json:"runId"`
	Service           string            `json:"service"`
	Stage             string            `json:"stage"`
	Provider          string            `json:"provider,omitempty"`
	WorkflowHash      string            `json:"workflowHash"`
	Entrypoint        string            `json:"entrypoint,omitempty"`
	Status            WorkflowRunStatus `json:"status"`
	StartedAt         string            `json:"startedAt"`
	EndedAt           string            `json:"endedAt,omitempty"`
	DurationMs        int64             `json:"durationMs,omitempty"`
	Nodes             []NodeRun         `json:"nodes,omitempty"`
	TotalInputTokens  int64             `json:"totalInputTokens,omitempty"`
	TotalOutputTokens int64             `json:"totalOutputTokens,omitempty"`
	EstimatedCostUSD  float64           `json:"estimatedCostUsd,omitempty"`
}

func runDir(root, stage string) string {
	return filepath.Join(root, ".runfabric", "runs", stage)
}

func runPath(root, stage, runID string) string {
	return filepath.Join(runDir(root, stage), runID+".json")
}

// SaveWorkflowRun persists a workflow run record under .runfabric/runs/<stage>/<runId>.json.
func SaveWorkflowRun(root string, run *WorkflowRun) error {
	if run == nil {
		return fmt.Errorf("nil workflow run")
	}
	if run.RunID == "" {
		return fmt.Errorf("runId is required")
	}
	if run.Stage == "" {
		return fmt.Errorf("stage is required")
	}
	dir := runDir(root, run.Stage)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create runs dir: %w", err)
	}
	data, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal workflow run: %w", err)
	}
	if err := os.WriteFile(runPath(root, run.Stage, run.RunID), data, 0o644); err != nil {
		return fmt.Errorf("write workflow run: %w", err)
	}
	return nil
}

// ListWorkflowRuns returns up to limit runs for the stage, sorted newest-first by StartedAt (best-effort).
func ListWorkflowRuns(root, stage string, limit int) ([]*WorkflowRun, error) {
	if limit <= 0 {
		limit = 20
	}
	dir := runDir(root, stage)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	type item struct {
		run *WorkflowRun
		t   time.Time
	}
	var items []item
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if filepath.Ext(name) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		var r WorkflowRun
		if err := json.Unmarshal(data, &r); err != nil {
			continue
		}
		t, _ := time.Parse(time.RFC3339, r.StartedAt)
		items = append(items, item{run: &r, t: t})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].t.After(items[j].t) })
	if len(items) > limit {
		items = items[:limit]
	}
	out := make([]*WorkflowRun, 0, len(items))
	for _, it := range items {
		out = append(out, it.run)
	}
	return out, nil
}

// WorkflowCostSummary aggregates token and cost across workflow runs (Phase 14.6).
type WorkflowCostSummary struct {
	RunCount          int     `json:"runCount"`
	TotalInputTokens  int64   `json:"totalInputTokens"`
	TotalOutputTokens int64   `json:"totalOutputTokens"`
	EstimatedCostUSD  float64 `json:"estimatedCostUsd"`
}

// WorkflowCostFromRuns computes a cost summary from the given runs (run-level totals, or sum of node tokens when run totals are zero).
func WorkflowCostFromRuns(runs []*WorkflowRun) WorkflowCostSummary {
	var s WorkflowCostSummary
	for _, r := range runs {
		s.RunCount++
		if r.TotalInputTokens != 0 || r.TotalOutputTokens != 0 || r.EstimatedCostUSD != 0 {
			s.TotalInputTokens += r.TotalInputTokens
			s.TotalOutputTokens += r.TotalOutputTokens
			s.EstimatedCostUSD += r.EstimatedCostUSD
		} else {
			for _, n := range r.Nodes {
				s.TotalInputTokens += n.InputTokens
				s.TotalOutputTokens += n.OutputTokens
				s.EstimatedCostUSD += n.EstimatedCostUSD
			}
		}
	}
	return s
}
