package recovery

import (
	"testing"
)

func TestRegistry_Register_Get(t *testing.T) {
	r := NewRegistry()
	factory := func(journal any) Handler { return nil }
	r.Register("aws", factory)
	h, err := r.Get("aws", nil)
	if err != nil {
		t.Fatal(err)
	}
	if h != nil {
		t.Errorf("expected nil handler from stub factory, got %v", h)
	}
}

func TestRegistry_Get_NotFound(t *testing.T) {
	r := NewRegistry()
	_, err := r.Get("unknown", nil)
	if err == nil {
		t.Fatal("expected error for unregistered provider")
	}
}
