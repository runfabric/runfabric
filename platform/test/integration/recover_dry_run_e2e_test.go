package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/runfabric/runfabric/platform/core/state/transactions"
	"github.com/runfabric/runfabric/platform/core/workflow/app"
)

func TestRecoverDryRunE2E(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "runfabric.yml")

	cfg := "service: recover-svc\n" +
		"provider:\n" +
		"  name: aws-lambda\n" +
		"  runtime: nodejs20.x\n" +
		"  region: ap-southeast-1\n" +
		"backend:\n" +
		"  kind: local\n" +
		"functions:\n" +
		"  - name: hello\n" +
		"    entry: src/handler.hello\n" +
		"    runtime: nodejs20.x\n"
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	backend := transactions.NewFileBackend(tmp)
	j := transactions.NewJournal("recover-svc", "dev", "deploy", backend)

	if err := j.Save(); err != nil {
		t.Fatalf("journal save failed: %v", err)
	}
	if err := j.Record(transactions.Operation{
		Type:     transactions.OpCreateLambda,
		Resource: "hello",
	}); err != nil {
		t.Fatalf("journal record failed: %v", err)
	}

	before, err := backend.Load("recover-svc", "dev")
	if err != nil {
		t.Fatalf("load before failed: %v", err)
	}

	result, err := app.RecoverDryRun(cfgPath, "dev")
	if err != nil {
		t.Fatalf("RecoverDryRun failed: %v", err)
	}

	m := toMapIntegration(t, result)
	if m["canRecover"] != true {
		t.Fatalf("expected canRecover=true, got %#v", m["canRecover"])
	}
	if m["checksumValid"] != true {
		t.Fatalf("expected checksumValid=true, got %#v", m["checksumValid"])
	}

	after, err := backend.Load("recover-svc", "dev")
	if err != nil {
		t.Fatalf("load after failed: %v", err)
	}

	if before.Version != after.Version {
		t.Fatalf("expected version unchanged, before=%d after=%d", before.Version, after.Version)
	}
	if before.Status != after.Status {
		t.Fatalf("expected status unchanged, before=%s after=%s", before.Status, after.Status)
	}
}

func toMapIntegration(t *testing.T, v any) map[string]any {
	t.Helper()

	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal result failed: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal result map failed: %v", err)
	}
	return m
}
