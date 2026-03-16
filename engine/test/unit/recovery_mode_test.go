package unit

import (
	"testing"

	"github.com/runfabric/runfabric/engine/internal/recovery"
)

func TestRecoveryModes(t *testing.T) {
	modes := []recovery.Mode{
		recovery.ModeRollback,
		recovery.ModeResume,
		recovery.ModeInspect,
	}

	if len(modes) != 3 {
		t.Fatal("expected 3 recovery modes")
	}
}
