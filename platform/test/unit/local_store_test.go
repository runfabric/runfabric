package unit

import (
	"os"
	"path/filepath"
	"testing"

	state "github.com/runfabric/runfabric/platform/core/state/core"
)

func TestSave_RequiresReceipt(t *testing.T) {
	root := t.TempDir()
	err := state.Save(root, nil)
	if err == nil {
		t.Fatal("expected error for nil receipt")
	}
}

func TestSave_AndLoad(t *testing.T) {
	root := t.TempDir()
	r := &state.Receipt{
		Service:      "svc",
		Stage:        "dev",
		Provider:     "aws-lambda",
		DeploymentID: "dep-1",
		Outputs:      map[string]string{"url": "https://example.com"},
	}
	if err := state.Save(root, r); err != nil {
		t.Fatal(err)
	}
	loaded, err := state.Load(root, "dev")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Service != "svc" || loaded.Stage != "dev" || loaded.DeploymentID != "dep-1" {
		t.Errorf("loaded receipt mismatch: %+v", loaded)
	}
	if loaded.Version != state.CurrentReceiptVersion {
		t.Errorf("version: got %d", loaded.Version)
	}
}

func TestLoad_NotFound(t *testing.T) {
	root := t.TempDir()
	_, err := state.Load(root, "nonexistent")
	if err == nil {
		t.Fatal("expected error for missing receipt")
	}
}

func TestDelete(t *testing.T) {
	root := t.TempDir()
	r := &state.Receipt{Service: "s", Stage: "dev", Provider: "aws-lambda", DeploymentID: "d"}
	if err := state.Save(root, r); err != nil {
		t.Fatal(err)
	}
	if err := state.Delete(root, "dev"); err != nil {
		t.Fatal(err)
	}
	_, err := state.Load(root, "dev")
	if err == nil {
		t.Fatal("expected load to fail after delete")
	}
}

func TestDelete_NoFile(t *testing.T) {
	root := t.TempDir()
	if err := state.Delete(root, "dev"); err != nil {
		t.Errorf("delete of missing file should succeed: %v", err)
	}
}

func TestListReleases_Empty(t *testing.T) {
	root := t.TempDir()
	list, err := state.ListReleases(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Errorf("expected no releases, got %d", len(list))
	}
}

func TestListReleases_WithReceipts(t *testing.T) {
	root := t.TempDir()
	for _, stage := range []string{"dev", "prod"} {
		r := &state.Receipt{Service: "s", Stage: stage, Provider: "aws-lambda", DeploymentID: stage}
		if err := state.Save(root, r); err != nil {
			t.Fatal(err)
		}
	}
	list, err := state.ListReleases(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 releases, got %d", len(list))
	}
}

func TestMigrateReceipt_Nil(t *testing.T) {
	_, err := state.MigrateReceipt(nil)
	if err == nil {
		t.Fatal("expected error for nil receipt")
	}
}

func TestMigrateReceipt_Version0(t *testing.T) {
	r := &state.Receipt{Version: 0, Service: "s", Stage: "dev"}
	out, err := state.MigrateReceipt(r)
	if err != nil {
		t.Fatal(err)
	}
	if out.Version != state.CurrentReceiptVersion {
		t.Errorf("version: got %d", out.Version)
	}
}

func TestLoad_LegacyFunctionFieldsMapToNeutralFields(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".runfabric"), 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := []byte(`{
	  "version": 1,
	  "service": "svc",
	  "stage": "dev",
	  "provider": "aws-lambda",
	  "deploymentId": "dep-1",
	  "outputs": {},
	  "functions": [
	    {
	      "function": "api",
	      "artifactSha256": "abc",
	      "configSignature": "cfg",
	      "lambdaName": "svc-dev-api",
	      "lambdaArn": "arn:aws:lambda:us-east-1:123:function:svc-dev-api"
	    }
	  ]
	}`)
	if err := os.WriteFile(filepath.Join(root, ".runfabric", "dev.json"), legacy, 0o644); err != nil {
		t.Fatal(err)
	}

	loaded, err := state.Load(root, "dev")
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Functions) != 1 {
		t.Fatalf("expected 1 function, got %d", len(loaded.Functions))
	}
	if loaded.Functions[0].ResourceName != "svc-dev-api" {
		t.Fatalf("expected resource name to migrate, got %q", loaded.Functions[0].ResourceName)
	}
	if loaded.Functions[0].ResourceIdentifier != "arn:aws:lambda:us-east-1:123:function:svc-dev-api" {
		t.Fatalf("expected resource identifier to migrate, got %q", loaded.Functions[0].ResourceIdentifier)
	}
	if loaded.Version != state.CurrentReceiptVersion {
		t.Fatalf("expected version=%d got=%d", state.CurrentReceiptVersion, loaded.Version)
	}
}

func TestMigrateReceipt_UnsupportedVersion(t *testing.T) {
	r := &state.Receipt{Version: 99, Service: "s", Stage: "dev"}
	_, err := state.MigrateReceipt(r)
	if err == nil {
		t.Fatal("expected error for unsupported version")
	}
}
