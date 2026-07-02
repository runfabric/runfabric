package runstore

import (
	"context"
	"errors"
	"testing"
	"time"

	core "github.com/runfabric/runfabric/platform/core/state/core"
)

func newRun(id string) *core.WorkflowRun {
	return &core.WorkflowRun{
		RunID:     id,
		Service:   "svc",
		Stage:     "dev",
		Status:    core.RunStatusRunning,
		StartedAt: time.Now().UTC().Format(time.RFC3339),
	}
}

func TestLocalStoreSaveLoadRoundTrip(t *testing.T) {
	s := NewLocalRunStore(t.TempDir())
	ctx := context.Background()

	if _, _, err := s.Load(ctx, "dev", "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Load(missing) = %v, want ErrNotFound", err)
	}

	v1, err := s.Save(ctx, newRun("r1"), "")
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if v1 == "" {
		t.Fatal("Save returned empty version")
	}

	got, v2, err := s.Load(ctx, "dev", "r1")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.RunID != "r1" {
		t.Fatalf("RunID = %q", got.RunID)
	}
	if v2 != v1 {
		t.Fatalf("Load version %s != Save version %s", v2, v1)
	}
}

func TestLocalStoreCASConflict(t *testing.T) {
	s := NewLocalRunStore(t.TempDir())
	ctx := context.Background()

	v1, err := s.Save(ctx, newRun("r1"), "")
	if err != nil {
		t.Fatalf("initial Save: %v", err)
	}

	// Simulate two readers that both loaded v1.
	runA, _, _ := s.Load(ctx, "dev", "r1")
	runB, _, _ := s.Load(ctx, "dev", "r1")

	// Writer A commits against v1 -> succeeds, producing v2.
	runA.Status = core.RunStatusOK
	v2, err := s.Save(ctx, runA, v1)
	if err != nil {
		t.Fatalf("writer A Save: %v", err)
	}
	if v2 == v1 {
		t.Fatal("expected a new version after write")
	}

	// Writer B commits against the stale v1 -> must conflict.
	runB.Status = core.RunStatusFailed
	if _, err := s.Save(ctx, runB, v1); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("writer B Save = %v, want ErrVersionConflict", err)
	}

	// Writer B reloads and retries against v2 -> succeeds.
	runB, cur, _ := s.Load(ctx, "dev", "r1")
	runB.Status = core.RunStatusFailed
	if _, err := s.Save(ctx, runB, cur); err != nil {
		t.Fatalf("writer B retry: %v", err)
	}
}

// A holder of the run Lock must still be able to Save (no reentrant deadlock),
// because the execution lock and the CAS lock are distinct.
func TestLockThenSaveDoesNotDeadlock(t *testing.T) {
	s := NewLocalRunStore(t.TempDir())
	ctx := context.Background()

	done := make(chan error, 1)
	go func() {
		release, err := s.Lock(ctx, "dev", "r1", time.Second)
		if err != nil {
			done <- err
			return
		}
		defer release()
		_, err = s.Save(ctx, newRun("r1"), "")
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("lock+save: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("deadlock: lock+save did not complete")
	}
}

func TestResolvePrecedence(t *testing.T) {
	root := t.TempDir()

	// No env, no config -> local default.
	s, err := Resolve("", root)
	if err != nil || s.Kind() != "local" {
		t.Fatalf("default Resolve = %v, %v; want local", s, err)
	}

	// Config value is used when env is unset.
	s, err = Resolve("dynamodb://runs?region=eu-west-1", root)
	if err != nil || s.Kind() != "dynamodb" {
		t.Fatalf("configured Resolve = %v, %v; want dynamodb", s, err)
	}

	// Env override wins over config.
	t.Setenv(EnvRunStore, "local")
	s, err = Resolve("dynamodb://runs", root)
	if err != nil || s.Kind() != "local" {
		t.Fatalf("env-override Resolve = %v, %v; want local", s, err)
	}

	// Selecting the store also yields its locker.
	if LockerFor(s) == nil {
		t.Fatal("local store should expose a RunLocker")
	}
}

func TestOpenSelectsBackend(t *testing.T) {
	local, err := Open("", t.TempDir())
	if err != nil || local.Kind() != "local" {
		t.Fatalf("Open(\"\") = %v, %v", local, err)
	}

	dyn, err := Open("dynamodb://runs?region=us-east-1", t.TempDir())
	if err != nil {
		t.Fatalf("Open(dynamodb) registry miss: %v", err)
	}
	if dyn.Kind() != "dynamodb" {
		t.Fatalf("Kind = %q, want dynamodb", dyn.Kind())
	}
	// The backend supplies its own distributed locker.
	if LockerFor(dyn) == nil {
		t.Fatal("dynamodb store should expose a RunLocker")
	}

	if _, err := Open("dynamodb://?region=us-east-1", t.TempDir()); err == nil {
		t.Fatal("Open(dynamodb without table) should error")
	}

	if _, err := Open("bogus://x", t.TempDir()); err == nil {
		t.Fatal("Open(unknown scheme) should error")
	}
}
