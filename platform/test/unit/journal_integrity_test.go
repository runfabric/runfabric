package unit

import (
	"testing"

	"github.com/runfabric/runfabric/platform/core/state/transactions"
)

func TestComputeChecksumStableForSameContent(t *testing.T) {
	j1 := &transactions.JournalFile{
		Service:      "svc",
		Stage:        "dev",
		Operation:    "deploy",
		Version:      1,
		AttemptCount: 0,
		Operations: []transactions.Operation{
			{
				Type:     transactions.OpCreateLambda,
				Resource: "hello",
			},
		},
	}

	j2 := &transactions.JournalFile{
		Service:      "svc",
		Stage:        "dev",
		Operation:    "deploy",
		Version:      1,
		AttemptCount: 0,
		Operations: []transactions.Operation{
			{
				Type:     transactions.OpCreateLambda,
				Resource: "hello",
			},
		},
	}

	c1, err := transactions.ComputeChecksum(j1)
	if err != nil {
		t.Fatalf("checksum 1 failed: %v", err)
	}
	c2, err := transactions.ComputeChecksum(j2)
	if err != nil {
		t.Fatalf("checksum 2 failed: %v", err)
	}

	if c1 != c2 {
		t.Fatalf("expected equal checksums, got %q vs %q", c1, c2)
	}
}

func TestVerifyChecksumPassesForValidJournal(t *testing.T) {
	j := &transactions.JournalFile{
		Service:   "svc",
		Stage:     "dev",
		Operation: "deploy",
		Version:   1,
	}

	sum, err := transactions.ComputeChecksum(j)
	if err != nil {
		t.Fatalf("compute checksum failed: %v", err)
	}
	j.Checksum = sum

	ok, err := transactions.VerifyChecksum(j)
	if err != nil {
		t.Fatalf("verify checksum failed: %v", err)
	}
	if !ok {
		t.Fatal("expected checksum verification to pass")
	}
}

func TestVerifyChecksumFailsForTamperedJournal(t *testing.T) {
	j := &transactions.JournalFile{
		Service:   "svc",
		Stage:     "dev",
		Operation: "deploy",
		Version:   1,
	}

	sum, err := transactions.ComputeChecksum(j)
	if err != nil {
		t.Fatalf("compute checksum failed: %v", err)
	}
	j.Checksum = sum

	// Tamper after checksum was computed.
	j.Stage = "prod"

	ok, err := transactions.VerifyChecksum(j)
	if err != nil {
		t.Fatalf("verify checksum failed: %v", err)
	}
	if ok {
		t.Fatal("expected checksum verification to fail after tampering")
	}
}

func TestJournalSaveSetsChecksum(t *testing.T) {
	tmp := t.TempDir()
	backend := transactions.NewFileBackend(tmp)

	j := transactions.NewJournal("svc", "dev", "deploy", backend)
	if err := j.Save(); err != nil {
		t.Fatalf("journal save failed: %v", err)
	}

	loaded, err := backend.Load("svc", "dev")
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if loaded.Checksum == "" {
		t.Fatal("expected checksum to be set after save")
	}

	ok, err := transactions.VerifyChecksum(loaded)
	if err != nil {
		t.Fatalf("verify checksum failed: %v", err)
	}
	if !ok {
		t.Fatal("expected saved journal checksum to be valid")
	}
}
