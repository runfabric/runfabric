package unit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/runfabric/runfabric/platform/core/state/transactions"
	"github.com/runfabric/runfabric/platform/workflow/app"
)

func TestRecoverDryRunNoJournal(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := writeMinimalConfig(t, tmp)

	result, err := app.RecoverDryRun(cfgPath, "dev")
	if err != nil {
		t.Fatalf("RecoverDryRun failed: %v", err)
	}

	m := toMap(t, result)
	if m["canRecover"] != false {
		t.Fatalf("expected canRecover=false, got %#v", m["canRecover"])
	}
}

func TestRecoverDryRunValidJournal(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := writeMinimalConfig(t, tmp)

	backend := transactions.NewFileBackend(tmp)
	j := transactions.NewJournal("test-svc", "dev", "deploy", backend)

	if err := j.Save(); err != nil {
		t.Fatalf("journal save failed: %v", err)
	}
	if err := j.Record(transactions.Operation{
		Type:     transactions.OpCreateLambda,
		Resource: "hello",
	}); err != nil {
		t.Fatalf("journal record failed: %v", err)
	}

	result, err := app.RecoverDryRun(cfgPath, "dev")
	if err != nil {
		t.Fatalf("RecoverDryRun failed: %v", err)
	}

	m := toMap(t, result)

	if m["canRecover"] != true {
		t.Fatalf("expected canRecover=true, got %#v", m["canRecover"])
	}
	if m["service"] != "test-svc" {
		t.Fatalf("expected service=test-svc, got %#v", m["service"])
	}
	if m["stage"] != "dev" {
		t.Fatalf("expected stage=dev, got %#v", m["stage"])
	}
	if m["checksumValid"] != true {
		t.Fatalf("expected checksumValid=true, got %#v", m["checksumValid"])
	}
}

func TestRecoverDryRunDetectsTamperedJournal(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := writeMinimalConfig(t, tmp)

	backend := transactions.NewFileBackend(tmp)
	j := transactions.NewJournal("test-svc", "dev", "deploy", backend)

	if err := j.Save(); err != nil {
		t.Fatalf("journal save failed: %v", err)
	}

	journalPath := filepath.Join(tmp, ".runfabric", "journals", "test-svc-dev.journal.json")
	data, err := os.ReadFile(journalPath)
	if err != nil {
		t.Fatalf("read journal file failed: %v", err)
	}

	// Tamper with raw journal JSON.
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal raw journal failed: %v", err)
	}
	raw["stage"] = "prod"

	tampered, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		t.Fatalf("marshal tampered journal failed: %v", err)
	}
	if err := os.WriteFile(journalPath, tampered, 0o644); err != nil {
		t.Fatalf("write tampered journal failed: %v", err)
	}

	result, err := app.RecoverDryRun(cfgPath, "dev")
	if err != nil {
		t.Fatalf("RecoverDryRun failed: %v", err)
	}

	m := toMap(t, result)
	if m["checksumValid"] != false {
		t.Fatalf("expected checksumValid=false, got %#v", m["checksumValid"])
	}
}

func TestRecoverDryRunDoesNotMutateState(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := writeMinimalConfig(t, tmp)

	backend := transactions.NewFileBackend(tmp)
	j := transactions.NewJournal("test-svc", "dev", "deploy", backend)

	if err := j.Save(); err != nil {
		t.Fatalf("journal save failed: %v", err)
	}

	before, err := backend.Load("test-svc", "dev")
	if err != nil {
		t.Fatalf("load before failed: %v", err)
	}

	_, err = app.RecoverDryRun(cfgPath, "dev")
	if err != nil {
		t.Fatalf("RecoverDryRun failed: %v", err)
	}

	after, err := backend.Load("test-svc", "dev")
	if err != nil {
		t.Fatalf("load after failed: %v", err)
	}

	if before.Version != after.Version {
		t.Fatalf("expected journal version unchanged, before=%d after=%d", before.Version, after.Version)
	}
	if before.Status != after.Status {
		t.Fatalf("expected journal status unchanged, before=%s after=%s", before.Status, after.Status)
	}
}

func writeMinimalConfig(t *testing.T, root string) string {
	t.Helper()

	cfg := "service: test-svc\n" +
		"provider:\n" +
		"  name: aws-lambda\n" +
		"  runtime: nodejs20.x\n" +
		"  region: ap-southeast-1\n" +
		"functions:\n" +
		"  - name: hello\n" +
		"    entry: src/handler.hello\n" +
		"    runtime: nodejs20.x\n"
	path := filepath.Join(root, "runfabric.yml")
	if err := os.WriteFile(path, []byte(cfg), 0o644); err != nil {
		t.Fatalf("write config failed: %v", err)
	}
	return path
}

func toMap(t *testing.T, v any) map[string]any {
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
