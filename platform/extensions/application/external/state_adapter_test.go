package external

import (
	"context"
	"testing"
	"time"

	statetypes "github.com/runfabric/runfabric/internal/state/types"
	"github.com/runfabric/runfabric/platform/state/backends"
)

func TestExternalStateBundleFactory_BasicOperations(t *testing.T) {
	exe := buildStubPlugin(t)
	factory := NewExternalStateBundleFactory("stub-state", "custom", exe)

	bundle, err := factory(context.Background(), backends.Options{})
	if err != nil {
		t.Fatalf("new external state bundle: %v", err)
	}
	if bundle == nil {
		t.Fatal("expected non-nil bundle")
	}

	handle, err := bundle.Locks.Acquire("svc", "dev", "deploy", time.Minute)
	if err != nil {
		t.Fatalf("acquire lock: %v", err)
	}
	if handle == nil || !handle.Held {
		t.Fatalf("unexpected lock handle: %#v", handle)
	}
	if err := handle.Release(); err != nil {
		t.Fatalf("release lock handle: %v", err)
	}

	journal, err := bundle.Journals.Load("svc", "dev")
	if err != nil {
		t.Fatalf("load journal: %v", err)
	}
	if journal.Stage != "dev" {
		t.Fatalf("journal stage=%q want dev", journal.Stage)
	}
	if err := bundle.Journals.Save(journal); err != nil {
		t.Fatalf("save journal: %v", err)
	}

	receipt, err := bundle.Receipts.Load("dev")
	if err != nil {
		t.Fatalf("load receipt: %v", err)
	}
	if receipt.Stage != "dev" {
		t.Fatalf("receipt stage=%q want dev", receipt.Stage)
	}
	if err := bundle.Receipts.Save(&statetypes.Receipt{Service: "svc", Stage: "dev"}); err != nil {
		t.Fatalf("save receipt: %v", err)
	}
	entries, err := bundle.Receipts.ListReleases()
	if err != nil {
		t.Fatalf("list releases: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected at least one release entry")
	}
	if bundle.Locks.Kind() != "custom" {
		t.Fatalf("lock backend kind=%q want custom", bundle.Locks.Kind())
	}
}
