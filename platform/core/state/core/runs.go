package core

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
	RunStatusPending   WorkflowRunStatus = "pending"
	RunStatusRunning   WorkflowRunStatus = "running"
	RunStatusPaused    WorkflowRunStatus = "paused"
	RunStatusOK        WorkflowRunStatus = "ok"
	RunStatusFailed    WorkflowRunStatus = "failed"
	RunStatusCancelled WorkflowRunStatus = "cancelled"
	RunStatusTimedOut  WorkflowRunStatus = "timed_out"
)

// NodeRunStatus is the status for a node run.
type NodeRunStatus string

const (
	NodeStatusRunning NodeRunStatus = "running"
	NodeStatusOK      NodeRunStatus = "ok"
	NodeStatusFailed  NodeRunStatus = "failed"
)

// WorkflowStepStatus is the durable status for one workflow step.
type WorkflowStepStatus string

const (
	StepStatusPending   WorkflowStepStatus = "pending"
	StepStatusRunning   WorkflowStepStatus = "running"
	StepStatusPaused    WorkflowStepStatus = "paused"
	StepStatusOK        WorkflowStepStatus = "ok"
	StepStatusFailed    WorkflowStepStatus = "failed"
	StepStatusCancelled WorkflowStepStatus = "cancelled"
	StepStatusTimedOut  WorkflowStepStatus = "timed_out"
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

// WorkflowStepRun is one durable step execution record.
type WorkflowStepRun struct {
	StepID       string             `json:"stepId"`
	Kind         string             `json:"kind,omitempty"`
	Input        map[string]any     `json:"input,omitempty"`
	Status       WorkflowStepStatus `json:"status"`
	AttemptCount int                `json:"attemptCount,omitempty"`
	MaxAttempts  int                `json:"maxAttempts,omitempty"`
	TimeoutMs    int64              `json:"timeoutMs,omitempty"`
	BackoffMs    int64              `json:"backoffMs,omitempty"`
	StartedAt    string             `json:"startedAt,omitempty"`
	EndedAt      string             `json:"endedAt,omitempty"`
	DurationMs   int64              `json:"durationMs,omitempty"`
	Error        string             `json:"error,omitempty"`
	Output       map[string]any     `json:"output,omitempty"`
	Metadata     map[string]any     `json:"metadata,omitempty"`
}

// WorkflowCheckpoint tracks resumable workflow position.
type WorkflowCheckpoint struct {
	CurrentStepID string `json:"currentStepId,omitempty"`
	CurrentStatus string `json:"currentStatus,omitempty"`
	UpdatedAt     string `json:"updatedAt,omitempty"`
	LastError     string `json:"lastError,omitempty"`
}

// WorkflowRun is one workflow run record.
// Phase 14.6: TotalInputTokens, TotalOutputTokens, EstimatedCostUSD aggregate per-run cost.
type WorkflowRun struct {
	RunID             string              `json:"runId"`
	Service           string              `json:"service"`
	Stage             string              `json:"stage"`
	Provider          string              `json:"provider,omitempty"`
	WorkflowName      string              `json:"workflowName,omitempty"`
	WorkflowHash      string              `json:"workflowHash"`
	Entrypoint        string              `json:"entrypoint,omitempty"`
	Status            WorkflowRunStatus   `json:"status"`
	StartedAt         string              `json:"startedAt"`
	EndedAt           string              `json:"endedAt,omitempty"`
	UpdatedAt         string              `json:"updatedAt,omitempty"`
	DurationMs        int64               `json:"durationMs,omitempty"`
	CancelRequested   bool                `json:"cancelRequested,omitempty"`
	ReplayFromStep    string              `json:"replayFromStep,omitempty"`
	Checkpoint        *WorkflowCheckpoint `json:"checkpoint,omitempty"`
	Steps             []WorkflowStepRun   `json:"steps,omitempty"`
	Nodes             []NodeRun           `json:"nodes,omitempty"`
	TotalInputTokens  int64               `json:"totalInputTokens,omitempty"`
	TotalOutputTokens int64               `json:"totalOutputTokens,omitempty"`
	EstimatedCostUSD  float64             `json:"estimatedCostUsd,omitempty"`
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
	run.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	data, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal workflow run: %w", err)
	}
	path := runPath(root, run.Stage, run.RunID)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write workflow run: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("move workflow run into place: %w", err)
	}
	return nil
}

// LoadWorkflowRun loads one run by stage and run id.
func LoadWorkflowRun(root, stage, runID string) (*WorkflowRun, error) {
	if stage == "" {
		return nil, fmt.Errorf("stage is required")
	}
	if runID == "" {
		return nil, fmt.Errorf("runId is required")
	}
	data, err := os.ReadFile(runPath(root, stage, runID))
	if err != nil {
		return nil, err
	}
	var run WorkflowRun
	if err := json.Unmarshal(data, &run); err != nil {
		return nil, fmt.Errorf("unmarshal workflow run: %w", err)
	}
	return &run, nil
}

// MarkWorkflowRunCancelRequested marks a run as cancel requested.
func MarkWorkflowRunCancelRequested(root, stage, runID string) error {
	run, err := LoadWorkflowRun(root, stage, runID)
	if err != nil {
		return err
	}
	run.CancelRequested = true
	return SaveWorkflowRun(root, run)
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

// IsWorkflowRunTerminal reports whether a run is in a terminal state.
func IsWorkflowRunTerminal(status WorkflowRunStatus) bool {
	switch status {
	case RunStatusOK, RunStatusFailed, RunStatusCancelled, RunStatusTimedOut:
		return true
	default:
		return false
	}
}

// IsWorkflowStepTerminal reports whether a step is in a terminal state.
func IsWorkflowStepTerminal(status WorkflowStepStatus) bool {
	switch status {
	case StepStatusOK, StepStatusFailed, StepStatusCancelled, StepStatusTimedOut:
		return true
	default:
		return false
	}
}
