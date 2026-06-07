package transactions

import (
	"testing"
)

// reloadAndVerify loads the persisted journal from the backend and asserts its
// stored checksum matches the bytes on disk.
func reloadAndVerify(t *testing.T, backend *FileBackend, service, stage string) {
	t.Helper()
	loaded, err := backend.Load(service, stage)
	if err != nil {
		t.Fatalf("load journal: %v", err)
	}
	ok, err := VerifyChecksum(loaded)
	if err != nil {
		t.Fatalf("verify checksum: %v", err)
	}
	if !ok {
		t.Fatalf("checksum mismatch after persist: stored=%q recomputed differs", loaded.Checksum)
	}
}

// TestJournal_ChecksumMatchesAfterEverySave guards the regression where the
// stored checksum never matched the persisted file: FileBackend.Save used to
// overwrite UpdatedAt after persist() computed the checksum, and the Mark*/
// Checkpoint/IncrementAttempt mutators wrote without recomputing it at all.
func TestJournal_ChecksumMatchesAfterEverySave(t *testing.T) {
	root := t.TempDir()
	backend := NewFileBackend(root)
	j := NewJournal("svc", "dev", "deploy", backend)

	if err := j.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}
	reloadAndVerify(t, backend, "svc", "dev")

	if err := j.Record(Operation{Type: OpCreateLambda, Resource: "fn1"}); err != nil {
		t.Fatalf("record: %v", err)
	}
	reloadAndVerify(t, backend, "svc", "dev")

	// Mutators that previously bypassed checksum recomputation entirely.
	if err := j.Checkpoint("deploy_functions", "done"); err != nil {
		t.Fatalf("checkpoint: %v", err)
	}
	reloadAndVerify(t, backend, "svc", "dev")

	if err := j.IncrementAttempt(); err != nil {
		t.Fatalf("increment attempt: %v", err)
	}
	reloadAndVerify(t, backend, "svc", "dev")

	if err := j.MarkCompleted(); err != nil {
		t.Fatalf("mark completed: %v", err)
	}
	reloadAndVerify(t, backend, "svc", "dev")
}
