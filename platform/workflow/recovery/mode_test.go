package recovery

import (
	"testing"
)

func TestMode_Constants(t *testing.T) {
	if ModeRollback != "rollback" || ModeResume != "resume" || ModeInspect != "inspect" {
		t.Errorf("Mode constants: rollback=%q resume=%q inspect=%q", ModeRollback, ModeResume, ModeInspect)
	}
}
