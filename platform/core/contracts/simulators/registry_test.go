package simulators_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	simulators "github.com/runfabric/runfabric/platform/core/contracts/simulators"
)

func TestBuiltinRegistry_LocalSimulator(t *testing.T) {
	reg := simulators.NewBuiltinRegistry()
	sim, err := reg.Get("local")
	if err != nil {
		t.Fatalf("get local simulator: %v", err)
	}
	res, err := sim.Simulate(context.Background(), simulators.Request{
		Service:  "svc",
		Stage:    "dev",
		Function: "api",
		Method:   "POST",
		Path:     "/api",
		Body:     []byte(`{"hello":"world"}`),
	})
	if err != nil {
		t.Fatalf("simulate: %v", err)
	}
	if res.StatusCode != 200 {
		t.Fatalf("status=%d want 200", res.StatusCode)
	}
	var body map[string]any
	if err := json.Unmarshal(res.Body, &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["function"] != "api" {
		t.Fatalf("body.function=%v want api", body["function"])
	}
}

func TestBuiltinRegistry_LocalSimulatorExecutesNodeHandler(t *testing.T) {
	workDir := t.TempDir()
	distDir := filepath.Join(workDir, "dist")
	if err := os.MkdirAll(distDir, 0o755); err != nil {
		t.Fatalf("mkdir dist: %v", err)
	}
	handlerPath := filepath.Join(distDir, "handler.js")
	handlerSource := "exports.handler = async (event) => ({ statusCode: 201, headers: { \"X-Test\": \"ok\" }, body: JSON.stringify({ message: \"Hello from test\", path: event.path, method: event.httpMethod }) });\n"
	if err := os.WriteFile(handlerPath, []byte(handlerSource), 0o644); err != nil {
		t.Fatalf("write handler: %v", err)
	}

	reg := simulators.NewBuiltinRegistry()
	sim, err := reg.Get("local")
	if err != nil {
		t.Fatalf("get local simulator: %v", err)
	}

	res, err := sim.Simulate(context.Background(), simulators.Request{
		Service:    "svc",
		Stage:      "dev",
		Function:   "handler",
		Method:     "GET",
		Path:       "/handler",
		WorkDir:    workDir,
		HandlerRef: "dist/handler.handler",
		Runtime:    "nodejs20.x",
	})
	if err != nil {
		t.Fatalf("simulate node handler: %v", err)
	}
	if res.StatusCode != 201 {
		t.Fatalf("status=%d want 201", res.StatusCode)
	}
	if res.Headers["X-Test"] != "ok" {
		t.Fatalf("header X-Test=%q want ok", res.Headers["X-Test"])
	}

	var body map[string]any
	if err := json.Unmarshal(res.Body, &body); err != nil {
		t.Fatalf("decode node handler body: %v", err)
	}
	if body["message"] != "Hello from test" {
		t.Fatalf("body.message=%v want Hello from test", body["message"])
	}
	if body["path"] != "/handler" {
		t.Fatalf("body.path=%v want /handler", body["path"])
	}
}
