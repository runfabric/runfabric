package unit

import (
	"testing"

	"github.com/runfabric/runfabric/engine/internal/state"
)

func TestStateSaveLoadDelete(t *testing.T) {
	tmp := t.TempDir()

	receipt := &state.Receipt{
		Service:      "hello-api",
		Stage:        "dev",
		Provider:     "aws",
		DeploymentID: "dep-123",
		Outputs: map[string]string{
			"hello": "https://example.com/hello",
		},
	}

	if err := state.Save(tmp, receipt); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	loaded, err := state.Load(tmp, "dev")
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if loaded.DeploymentID != "dep-123" {
		t.Fatalf("unexpected deployment id: %s", loaded.DeploymentID)
	}

	if err := state.Delete(tmp, "dev"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
}
