package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

func TestValidateConfigRejectsUnsupportedRuntime(t *testing.T) {
	p := newPlugin()
	err := p.ValidateConfig(context.Background(), sdkprovider.ValidateConfigRequest{
		Config: sdkprovider.Config{
			"service": "svc",
			"functions": []any{
				map[string]any{"name": "api", "runtime": "ruby3.3"},
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "unsupported runtime") {
		t.Fatalf("expected unsupported runtime error, got %v", err)
	}
}

func TestDoctorCallsLinodeProfileAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("unexpected auth header: %s", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"username": "linode-user", "email": "user@example.com"})
	}))
	defer server.Close()

	p := newPlugin()
	p.apiBaseURL = server.URL
	p.getenv = func(key string) string {
		if key == defaultTokenEnv {
			return "test-token"
		}
		return ""
	}

	result, err := p.Doctor(context.Background(), sdkprovider.DoctorRequest{Config: sdkprovider.Config{"service": "svc"}})
	if err != nil {
		t.Fatalf("doctor failed: %v", err)
	}
	joined := strings.Join(result.Checks, "\n")
	if !strings.Contains(joined, "authenticated to Linode API as linode-user") {
		t.Fatalf("expected auth check, got %v", result.Checks)
	}
}

func TestPlanSummarizesFunctions(t *testing.T) {
	p := newPlugin()
	result, err := p.Plan(context.Background(), sdkprovider.PlanRequest{
		Stage: "dev",
		Root:  "/tmp/project",
		Config: sdkprovider.Config{
			"service": "svc",
			"functions": []any{
				map[string]any{"name": "api", "runtime": "nodejs20.x", "entry": "src/api.handler", "triggers": []any{map[string]any{"type": "http"}}},
			},
		},
	})
	if err != nil {
		t.Fatalf("plan failed: %v", err)
	}
	planMap, ok := result.Plan.(map[string]any)
	if !ok {
		t.Fatalf("expected plan map, got %T", result.Plan)
	}
	actions, ok := planMap["actions"].([]map[string]any)
	if !ok || len(actions) != 1 {
		t.Fatalf("expected one action, got %#v", planMap["actions"])
	}
	if actions[0]["runtime"] != "nodejs" {
		t.Fatalf("expected normalized runtime, got %#v", actions[0]["runtime"])
	}
}

func TestInvokeUsesHTTPURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/invoke" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("X-Request-Id", "req-123")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	p := newPlugin()
	result, err := p.Invoke(context.Background(), sdkprovider.InvokeRequest{
		Function: "api",
		Payload:  []byte(`{"name":"test"}`),
		Config: sdkprovider.Config{
			"service": "svc",
			"functions": []any{
				map[string]any{"name": "api", "runtime": "nodejs20.x", "url": server.URL + "/invoke"},
			},
		},
	})
	if err != nil {
		t.Fatalf("invoke failed: %v", err)
	}
	if result.RunID != "req-123" {
		t.Fatalf("expected run id, got %#v", result)
	}
	if result.Output != `{"ok":true}` {
		t.Fatalf("unexpected output: %s", result.Output)
	}
}

func TestDeployParsesCommandJSON(t *testing.T) {
	p := newPlugin()
	p.getenv = func(string) string { return "" }
	p.runCommand = func(ctx context.Context, cwd, command string, env []string) ([]byte, error) {
		_ = ctx
		_ = cwd
		_ = command
		_ = env
		return []byte(`{"deploymentId":"dep-123","outputs":{"endpoint":"https://example.test"}}`), nil
	}
	p.deploymentNow = func() time.Time { return time.Unix(1700000000, 0).UTC() }

	result, err := p.Deploy(context.Background(), sdkprovider.DeployRequest{
		Stage: "dev",
		Root:  "/tmp/project",
		Config: sdkprovider.Config{
			"service":       "svc",
			"runtime":       "nodejs20.x",
			"deployCommand": "echo ok",
			"functions":     []any{map[string]any{"name": "api", "runtime": "nodejs20.x"}},
		},
	})
	if err != nil {
		t.Fatalf("deploy failed: %v", err)
	}
	if result.DeploymentID != "dep-123" {
		t.Fatalf("expected parsed deployment id, got %#v", result)
	}
	if result.Outputs["endpoint"] != "https://example.test" {
		t.Fatalf("expected parsed endpoint, got %#v", result.Outputs)
	}
	if result.Provider != "linode" {
		t.Fatalf("expected provider linode, got %#v", result.Provider)
	}
}

func TestResolveCommandUsesBuiltInLinodeCLIFallbacks(t *testing.T) {
	p := newPlugin()
	p.getenv = func(key string) string {
		if key == defaultCLIBinEnv {
			return "/opt/homebrew/bin/linode-cli"
		}
		return ""
	}

	removeCmd := p.resolveCommand(sdkprovider.Config{}, "remove")
	logsCmd := p.resolveCommand(sdkprovider.Config{}, "logs")

	if !strings.Contains(removeCmd, "/opt/homebrew/bin/linode-cli") || !strings.Contains(removeCmd, "functions action-delete") {
		t.Fatalf("unexpected remove fallback: %s", removeCmd)
	}
	if !strings.Contains(logsCmd, "/opt/homebrew/bin/linode-cli") || !strings.Contains(logsCmd, "functions activation-list") {
		t.Fatalf("unexpected logs fallback: %s", logsCmd)
	}
}

func TestPlanTreatsBuiltInCLICommandsAsAvailable(t *testing.T) {
	p := newPlugin()
	p.getenv = func(key string) string {
		if key == defaultTokenEnv {
			return "token"
		}
		return ""
	}

	result, err := p.Plan(context.Background(), sdkprovider.PlanRequest{
		Config: sdkprovider.Config{
			"service":   "svc",
			"runtime":   "nodejs20.x",
			"functions": []any{map[string]any{"name": "api", "runtime": "nodejs20.x"}},
		},
	})
	if err != nil {
		t.Fatalf("plan failed: %v", err)
	}
	warnings := strings.Join(result.Warnings, "\n")
	if strings.Contains(warnings, removeCommandEnv) {
		t.Fatalf("expected remove warning to be suppressed by built-in fallback, got %v", result.Warnings)
	}
	if strings.Contains(warnings, logsCommandEnv) {
		t.Fatalf("expected logs warning to be suppressed by built-in fallback, got %v", result.Warnings)
	}
	if !strings.Contains(warnings, deployCommandEnv) {
		t.Fatalf("expected deploy warning to remain, got %v", result.Warnings)
	}
}

func TestPlanUsesDeployConventionWhenAppIDAndArtifactExist(t *testing.T) {
	p := newPlugin()
	p.getenv = func(key string) string {
		if key == defaultTokenEnv {
			return "token"
		}
		return ""
	}

	result, err := p.Plan(context.Background(), sdkprovider.PlanRequest{
		Root: "/workspace/service",
		Config: sdkprovider.Config{
			"service": "svc",
			"appID":   "1234",
			"functions": []any{
				map[string]any{"name": "api", "runtime": "nodejs20.x", "artifact": "dist/api.zip"},
			},
		},
	})
	if err != nil {
		t.Fatalf("plan failed: %v", err)
	}
	warnings := strings.Join(result.Warnings, "\n")
	if strings.Contains(warnings, deployCommandEnv) {
		t.Fatalf("expected deploy warning to be suppressed, got %v", result.Warnings)
	}
	planMap := result.Plan.(map[string]any)
	actions := planMap["actions"].([]map[string]any)
	if actions[0]["artifact"] != "dist/api.zip" {
		t.Fatalf("expected artifact to be reported in plan, got %#v", actions[0])
	}
}

func TestExecuteOperationExportsArtifactContext(t *testing.T) {
	p := newPlugin()
	p.getenv = func(key string) string {
		if key == defaultTokenEnv {
			return "token"
		}
		return ""
	}
	var captured []string
	p.runCommand = func(ctx context.Context, cwd, command string, env []string) ([]byte, error) {
		_ = ctx
		_ = cwd
		_ = command
		captured = append([]string(nil), env...)
		return []byte(`{"removed":true}`), nil
	}

	_, err := p.Remove(context.Background(), sdkprovider.RemoveRequest{
		Root: "/workspace/service",
		Config: sdkprovider.Config{
			"service": "svc",
			"functions": []any{
				map[string]any{"name": "api", "runtime": "nodejs20.x", "artifact": "dist/api.zip"},
			},
		},
	})
	if err != nil {
		t.Fatalf("remove failed: %v", err)
	}
	joined := strings.Join(captured, "\n")
	if !strings.Contains(joined, "RUNFABRIC_ARTIFACT_PATH=/workspace/service/dist/api.zip") {
		t.Fatalf("expected artifact path in env, got %v", captured)
	}
	if !strings.Contains(joined, "RUNFABRIC_ARTIFACT_BASENAME=api.zip") {
		t.Fatalf("expected artifact basename in env, got %v", captured)
	}
}
