package locking

import (
	"testing"
	"time"
)

func TestNewHandle(t *testing.T) {
	h := NewHandle("svc", "dev", "token", nil, nil, nil)
	if h == nil || h.Service != "svc" || h.Stage != "dev" || h.OwnerToken != "token" || !h.Held {
		t.Errorf("NewHandle: %+v", h)
	}
}

func TestHandle_Release_NilOrNotHeld(t *testing.T) {
	var h *Handle
	if err := h.Release(); err != nil {
		t.Errorf("nil Release: %v", err)
	}
	h = &Handle{Held: false}
	if err := h.Release(); err != nil {
		t.Errorf("Release when not held: %v", err)
	}
}

func TestHandle_Renew_NilOrNotHeld(t *testing.T) {
	var h *Handle
	if err := h.Renew(time.Minute); err != nil {
		t.Errorf("nil Renew: %v", err)
	}
	h = &Handle{Held: false}
	if err := h.Renew(time.Minute); err != nil {
		t.Errorf("Renew when not held: %v", err)
	}
	h = &Handle{Held: true, n: nil}
	if err := h.Renew(time.Minute); err != nil {
		t.Errorf("Renew when Renewer nil: %v", err)
	}
}
