package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStateList_LocalBackend(t *testing.T) {
	tmp := t.TempDir()
	cfg := "service: state-svc\n" +
		"provider:\n" +
		"  name: aws-lambda\n" +
		"  runtime: nodejs20.x\n" +
		"  region: us-east-1\n" +
		"backend:\n" +
		"  kind: local\n" +
		"functions:\n" +
		"  - name: hello\n" +
		"    entry: src/handler.hello\n" +
		"    runtime: nodejs20.x\n"
	cfgPath := filepath.Join(tmp, "runfabric.yml")
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	result, err := StateList(cfgPath, "dev")
	if err != nil {
		t.Fatalf("StateList: %v", err)
	}
	out, ok := result.(*StateListResult)
	if !ok {
		t.Fatalf("expected *StateListResult, got %T", result)
	}
	if out.Service != "state-svc" {
		t.Errorf("service: got %q", out.Service)
	}
	if out.Backend != "local" {
		t.Errorf("backend: got %q", out.Backend)
	}
	if len(out.Releases) > 0 {
		t.Errorf("expected empty releases, got %d", len(out.Releases))
	}
}

func TestStateBackupRestore_LocalBackend(t *testing.T) {
	tmp := t.TempDir()
	cfg := "service: state-svc\n" +
		"provider:\n" +
		"  name: aws-lambda\n" +
		"  runtime: nodejs20.x\n" +
		"  region: us-east-1\n" +
		"backend:\n" +
		"  kind: local\n" +
		"functions:\n" +
		"  - name: hello\n" +
		"    entry: src/handler.hello\n" +
		"    runtime: nodejs20.x\n"
	cfgPath := filepath.Join(tmp, "runfabric.yml")
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	backupPath := filepath.Join(tmp, "backup.json")
	_, err := StateBackup(cfgPath, "dev", backupPath)
	if err != nil {
		t.Fatalf("StateBackup: %v", err)
	}
	if _, err := os.Stat(backupPath); err != nil {
		t.Fatalf("backup file not created: %v", err)
	}

	_, err = StateRestore(cfgPath, "dev", backupPath)
	if err != nil {
		t.Fatalf("StateRestore: %v", err)
	}
}
