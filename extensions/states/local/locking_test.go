package local

import (
	"testing"
)

func TestLockBackend_New_AndKind(t *testing.T) {
	root := t.TempDir()
	b := NewLockBackend(root)
	if b == nil {
		t.Fatal("NewLockBackend returned nil")
	}
	if b.Kind() != "local" {
		t.Errorf("Kind: got %q", b.Kind())
	}
	if b.Root != root {
		t.Fatalf("Root: got %q want %q", b.Root, root)
	}
}
