package unit

import (
	"context"
	"testing"

	awsprovider "github.com/runfabric/runfabric/engine/internal/extensions/provider/aws"
	"github.com/runfabric/runfabric/engine/internal/transactions"
)

func TestRecoveryDetectsUnfinishedJournal(t *testing.T) {
	tmp := t.TempDir()

	backend := transactions.NewFileBackend(tmp)
	j := transactions.NewJournal("svc", "dev", "deploy", backend)

	if err := j.Save(); err != nil {
		t.Fatalf("save journal failed: %v", err)
	}

	// We can't load real AWS clients here, so this is more of a backend sanity test.
	_ = context.Background()
	_ = awsprovider.New()

	loaded, err := backend.Load("svc", "dev")
	if err != nil {
		t.Fatalf("load journal failed: %v", err)
	}

	if loaded.Status != transactions.StatusActive {
		t.Fatalf("expected active status, got %s", loaded.Status)
	}
}
