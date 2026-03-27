package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
)

type testPlugin struct{}

func (p *testPlugin) Meta() Meta {
	return Meta{
		Name:            "acme-provider",
		Version:         "0.2.0",
		SupportsRuntime: []string{"nodejs"},
		SupportsTriggers: []string{
			"http",
		},
	}
}

func (p *testPlugin) ValidateConfig(ctx context.Context, req ValidateConfigRequest) error {
	return nil
}

func (p *testPlugin) Doctor(ctx context.Context, req DoctorRequest) (*DoctorResult, error) {
	return &DoctorResult{Provider: "acme-provider", Checks: []string{"ok"}}, nil
}

func (p *testPlugin) Plan(ctx context.Context, req PlanRequest) (*PlanResult, error) {
	return &PlanResult{Provider: "acme-provider", Plan: map[string]any{"actions": []any{}}, Warnings: []string{}}, nil
}

func (p *testPlugin) Deploy(ctx context.Context, req DeployRequest) (*DeployResult, error) {
	return &DeployResult{Provider: "acme-provider", DeploymentID: "deploy-1", Outputs: map[string]string{}}, nil
}

func (p *testPlugin) Remove(ctx context.Context, req RemoveRequest) (*RemoveResult, error) {
	return &RemoveResult{Provider: "acme-provider", Removed: true}, nil
}

func (p *testPlugin) Invoke(ctx context.Context, req InvokeRequest) (*InvokeResult, error) {
	return &InvokeResult{Provider: "acme-provider", Function: req.Function, Output: "ok"}, nil
}

func (p *testPlugin) Logs(ctx context.Context, req LogsRequest) (*LogsResult, error) {
	return &LogsResult{Provider: "acme-provider", Function: req.Function, Lines: []string{"line"}}, nil
}

func (p *testPlugin) FetchMetrics(ctx context.Context, req MetricsRequest) (*MetricsResult, error) {
	return &MetricsResult{Message: "metrics"}, nil
}

func (p *testPlugin) FetchTraces(ctx context.Context, req TracesRequest) (*TracesResult, error) {
	return &TracesResult{Message: "traces"}, nil
}

func (p *testPlugin) PrepareDevStream(ctx context.Context, req DevStreamRequest) (*DevStreamSession, error) {
	return &DevStreamSession{EffectiveMode: "route-rewrite", StatusMessage: "prepared"}, nil
}

func (p *testPlugin) Recover(ctx context.Context, req RecoveryRequest) (*RecoveryResult, error) {
	return &RecoveryResult{Recovered: true, Mode: "resume", Status: "ok", Message: "recovered"}, nil
}

func (p *testPlugin) SyncOrchestrations(ctx context.Context, req OrchestrationSyncRequest) (*OrchestrationSyncResult, error) {
	return &OrchestrationSyncResult{Outputs: map[string]string{"orchestration": "synced"}}, nil
}

func (p *testPlugin) RemoveOrchestrations(ctx context.Context, req OrchestrationRemoveRequest) (*OrchestrationSyncResult, error) {
	return &OrchestrationSyncResult{Outputs: map[string]string{"orchestration": "removed"}}, nil
}

func (p *testPlugin) InvokeOrchestration(ctx context.Context, req OrchestrationInvokeRequest) (*InvokeResult, error) {
	return &InvokeResult{Provider: "acme-provider", Output: "orchestration-invoked"}, nil
}

func (p *testPlugin) InspectOrchestrations(ctx context.Context, req OrchestrationInspectRequest) (map[string]any, error) {
	return map[string]any{"count": 1, "status": "ok"}, nil
}

func TestNewServer_TypedProviderDispatch(t *testing.T) {
	pl := &testPlugin{}
	s := NewServer(pl, ServeOptions{ProtocolVersion: "1"})
	in := bytes.NewBufferString(
		`{"id":"1","method":"Handshake"}` + "\n" +
			`{"id":"2","method":"Doctor","params":{"stage":"dev"}}` + "\n" +
			`{"id":"3","method":"FetchMetrics","params":{"stage":"dev"}}` + "\n" +
			`{"id":"4","method":"PrepareDevStream","params":{"stage":"dev","tunnelURL":"https://example"}}` + "\n" +
			`{"id":"5","method":"Recover","params":{"mode":"resume"}}` + "\n" +
			`{"id":"6","method":"SyncOrchestrations","params":{"stage":"dev"}}` + "\n" +
			`{"id":"7","method":"InspectOrchestrations","params":{"stage":"dev"}}` + "\n",
	)
	var out bytes.Buffer
	if err := s.Serve(context.Background(), in, &out); err != nil {
		t.Fatalf("serve: %v", err)
	}

	lines := bytes.Split(bytes.TrimSpace(out.Bytes()), []byte("\n"))
	if len(lines) != 7 {
		t.Fatalf("expected 7 responses, got %d", len(lines))
	}

	var hs map[string]any
	if err := json.Unmarshal(lines[0], &hs); err != nil {
		t.Fatalf("decode handshake response: %v", err)
	}
	result := hs["result"].(map[string]any)
	if result["protocolVersion"] != "1" {
		t.Fatalf("protocolVersion mismatch: %#v", result)
	}
	if result["version"] != "0.2.0" {
		t.Fatalf("version mismatch: %#v", result)
	}
	caps := result["capabilities"].([]any)
	if len(caps) < 10 {
		t.Fatalf("expected typed capabilities in handshake, got %#v", caps)
	}

	var doctor map[string]any
	if err := json.Unmarshal(lines[1], &doctor); err != nil {
		t.Fatalf("decode doctor response: %v", err)
	}
	doctorResult := doctor["result"].(map[string]any)
	if doctorResult["provider"] != "acme-provider" {
		t.Fatalf("doctor provider mismatch: %#v", doctorResult)
	}

	var metrics map[string]any
	if err := json.Unmarshal(lines[2], &metrics); err != nil {
		t.Fatalf("decode metrics response: %v", err)
	}
	metricsResult := metrics["result"].(map[string]any)
	if metricsResult["message"] != "metrics" {
		t.Fatalf("metrics message mismatch: %#v", metricsResult)
	}

	var devStream map[string]any
	if err := json.Unmarshal(lines[3], &devStream); err != nil {
		t.Fatalf("decode dev-stream response: %v", err)
	}
	devResult := devStream["result"].(map[string]any)
	if devResult["effectiveMode"] != "route-rewrite" {
		t.Fatalf("dev-stream mode mismatch: %#v", devResult)
	}

	var recoverResp map[string]any
	if err := json.Unmarshal(lines[4], &recoverResp); err != nil {
		t.Fatalf("decode recover response: %v", err)
	}
	recoverResult := recoverResp["result"].(map[string]any)
	if recoverResult["mode"] != "resume" || recoverResult["status"] != "ok" {
		t.Fatalf("recover response mismatch: %#v", recoverResult)
	}

	var syncResp map[string]any
	if err := json.Unmarshal(lines[5], &syncResp); err != nil {
		t.Fatalf("decode sync orchestration response: %v", err)
	}
	syncResult := syncResp["result"].(map[string]any)
	outputs := syncResult["outputs"].(map[string]any)
	if outputs["orchestration"] != "synced" {
		t.Fatalf("sync orchestration response mismatch: %#v", syncResult)
	}

	var inspectResp map[string]any
	if err := json.Unmarshal(lines[6], &inspectResp); err != nil {
		t.Fatalf("decode inspect orchestration response: %v", err)
	}
	inspectResult := inspectResp["result"].(map[string]any)
	if inspectResult["status"] != "ok" {
		t.Fatalf("inspect orchestration response mismatch: %#v", inspectResult)
	}
}
