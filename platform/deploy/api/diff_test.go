package api

import (
	"testing"

	contracts "github.com/runfabric/runfabric/platform/core/contracts/provider"
	"github.com/runfabric/runfabric/platform/core/model/config"
	state "github.com/runfabric/runfabric/platform/core/state/core"
)

func opFor(cs *contracts.Changeset, name string) contracts.ChangeOp {
	for _, ch := range cs.Functions {
		if ch.Name == name {
			return ch.Op
		}
	}
	return ""
}

func testConfig(fn config.FunctionConfig) *config.Config {
	return &config.Config{
		Service:   "svc",
		Provider:  config.ProviderConfig{Name: "aws-lambda", Runtime: "python3.12"},
		Functions: map[string]config.FunctionConfig{"api": fn},
	}
}

// TestComputeChangeset_NoOpWhenConfigUnchanged guards the regression where the
// desired fingerprint and the receipt-derived state shared no comparison keys,
// making ChangeOpNoOp unreachable (every function was always reported as an
// update).
func TestComputeChangeset_NoOpWhenConfigUnchanged(t *testing.T) {
	root := t.TempDir()
	fn := config.FunctionConfig{Runtime: "python3.12", Handler: "app.handler", Memory: 256}

	sig, err := config.FunctionConfigSignature(fn)
	if err != nil {
		t.Fatalf("signature: %v", err)
	}
	if err := state.Save(root, &state.Receipt{
		Service:  "svc",
		Stage:    "dev",
		Provider: "aws-lambda",
		Outputs:  map[string]string{},
		Functions: []state.FunctionDeployment{
			{Function: "api", ConfigSignature: sig},
		},
	}); err != nil {
		t.Fatalf("save receipt: %v", err)
	}

	cs := computeChangeset(testConfig(fn), "dev", root)
	if op := opFor(cs, "api"); op != contracts.ChangeOpNoOp {
		t.Fatalf("op = %q, want no-op for an unchanged function", op)
	}
}

// TestComputeChangeset_UpdateWhenConfigChanged verifies a changed config (here
// memory) is reported as an update.
func TestComputeChangeset_UpdateWhenConfigChanged(t *testing.T) {
	root := t.TempDir()
	deployed := config.FunctionConfig{Runtime: "python3.12", Handler: "app.handler", Memory: 256}
	sig, err := config.FunctionConfigSignature(deployed)
	if err != nil {
		t.Fatalf("signature: %v", err)
	}
	if err := state.Save(root, &state.Receipt{
		Service:  "svc",
		Stage:    "dev",
		Provider: "aws-lambda",
		Outputs:  map[string]string{},
		Functions: []state.FunctionDeployment{
			{Function: "api", ConfigSignature: sig},
		},
	}); err != nil {
		t.Fatalf("save receipt: %v", err)
	}

	changed := deployed
	changed.Memory = 512
	cs := computeChangeset(testConfig(changed), "dev", root)
	if op := opFor(cs, "api"); op != contracts.ChangeOpUpdate {
		t.Fatalf("op = %q, want update for a changed function", op)
	}
}

// TestComputeChangeset_CreateWhenNoReceipt verifies that with no prior receipt
// every function is a create.
func TestComputeChangeset_CreateWhenNoReceipt(t *testing.T) {
	root := t.TempDir()
	fn := config.FunctionConfig{Runtime: "python3.12", Handler: "app.handler"}
	cs := computeChangeset(testConfig(fn), "dev", root)
	if op := opFor(cs, "api"); op != contracts.ChangeOpCreate {
		t.Fatalf("op = %q, want create when no receipt exists", op)
	}
}
