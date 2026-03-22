package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/runfabric/runfabric/platform/core/model/config"
	"github.com/runfabric/runfabric/platform/extensions/registry/resolution"
)

func TestResolveSimulatorForLocal_DefaultLocal(t *testing.T) {
	b, err := resolution.New(resolution.Options{IncludeExternal: false})
	if err != nil {
		t.Fatalf("new boundary: %v", err)
	}
	ctx := &AppContext{Config: &config.Config{}, Extensions: b}

	sim, err := resolveSimulatorForLocal(ctx)
	if err != nil {
		t.Fatalf("resolve simulator: %v", err)
	}
	if sim.Meta().ID != "local" {
		t.Fatalf("simulator id=%q want local", sim.Meta().ID)
	}
}

func TestLocalInvokeHandler_UsesSimulatorPlugin(t *testing.T) {
	b, err := resolution.New(resolution.Options{IncludeExternal: false})
	if err != nil {
		t.Fatalf("new boundary: %v", err)
	}
	ctx := &AppContext{
		Config: &config.Config{
			Service: "svc",
			Extensions: map[string]any{
				"simulatorPlugin": "local",
			},
		},
		Stage:      "dev",
		Extensions: b,
	}

	sim, err := resolveSimulatorForLocal(ctx)
	if err != nil {
		t.Fatalf("resolve simulator: %v", err)
	}

	h := newLocalInvokeHandler(ctx, sim, "api")
	req := httptest.NewRequest(http.MethodPost, "/api?x=1", strings.NewReader(`{"ok":true}`))
	req.Header.Set("X-Test", "yes")
	rr := httptest.NewRecorder()

	h(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d want 200", rr.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["function"] != "api" {
		t.Fatalf("function=%v want api", body["function"])
	}
	if body["service"] != "svc" {
		t.Fatalf("service=%v want svc", body["service"])
	}
}
