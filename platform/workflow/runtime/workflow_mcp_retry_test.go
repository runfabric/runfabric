package runtime

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/runfabric/runfabric/platform/core/model/config"
	state "github.com/runfabric/runfabric/platform/core/state/core"
)

// flakyMCPClient fails the first failFirst calls of each method, then succeeds.
type flakyMCPClient struct {
	failFirst int
	calls     int
}

func (c *flakyMCPClient) CallTool(_ context.Context, _, _ string, _ map[string]any) (map[string]any, error) {
	c.calls++
	if c.calls <= c.failFirst {
		return nil, errors.New("transient")
	}
	return map[string]any{"ok": true}, nil
}
func (c *flakyMCPClient) ReadResource(_ context.Context, _, _ string) (map[string]any, error) {
	return map[string]any{}, nil
}
func (c *flakyMCPClient) GetPrompt(_ context.Context, _, _ string, _ map[string]any) (map[string]any, error) {
	return map[string]any{}, nil
}

// TestMCPRuntime_RetryRecordsAuditMetadataOnce guards the regression where
// retrying an MCP call re-ran policy enforcement and correlation recording on
// every attempt, duplicating the audit metadata.
func TestMCPRuntime_RetryRecordsAuditMetadataOnce(t *testing.T) {
	client := &flakyMCPClient{failFirst: 1}
	rt := NewMCPRuntime(
		client,
		config.MCPIntegrationsConfig{Servers: map[string]config.MCPServerConfig{"db": {URL: "http://db"}}},
		config.MCPPolicyConfig{},
	)
	metadata := map[string]any{}
	b := MCPBinding{Server: "db", Tool: "read"}

	_, err := rt.CallTool(context.Background(), &state.WorkflowRun{}, state.WorkflowStepRun{}, b, metadata,
		DefaultRetryStrategy{MaxAttempts: 3, BaseBackoff: time.Nanosecond})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}

	if client.calls != 2 {
		t.Errorf("client calls = %d, want 2 (one failure + one success)", client.calls)
	}
	if calls, _ := metadata["mcpCalls"].([]any); len(calls) != 1 {
		t.Errorf("mcpCalls records = %d, want 1 (retry must not duplicate)", len(calls))
	}
	if pol, _ := metadata["mcpPolicy"].([]any); len(pol) != 1 {
		t.Errorf("mcpPolicy records = %d, want 1 (retry must not duplicate)", len(pol))
	}
}
