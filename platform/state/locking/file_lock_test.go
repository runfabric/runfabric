package locking

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAcquire_Release(t *testing.T) {
	root := t.TempDir()
	lock, err := Acquire(root, "svc", "dev")
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if !lock.Held || lock.Path == "" {
		t.Errorf("lock not held or path empty")
	}
	_, err = Acquire(root, "svc", "dev")
	if err == nil {
		t.Fatal("second Acquire should fail")
	}
	if err := lock.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}
	if lock.Held {
		t.Error("Release should set Held=false")
	}
	lock2, err := Acquire(root, "svc", "dev")
	if err != nil {
		t.Fatalf("Acquire after release: %v", err)
	}
	_ = lock2.Release()
}

func TestFileLock_Release_NilOrNotHeld(t *testing.T) {
	var l *FileLock
	if err := l.Release(); err != nil {
		t.Errorf("nil Release should return nil: %v", err)
	}
	l = &FileLock{Path: "/nonexistent", Held: false}
	if err := l.Release(); err != nil {
		t.Errorf("Release when not held: %v", err)
	}
}

func TestAcquire_CreatesLockDir(t *testing.T) {
	root := t.TempDir()
	_, err := Acquire(root, "s", "st")
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	lockDir := filepath.Join(root, ".runfabric", "locks")
	if _, err := os.Stat(lockDir); os.IsNotExist(err) {
		t.Error("lock dir should exist")
	}
}
