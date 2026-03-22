package simulators_test

import (
	"context"
	"encoding/json"
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
