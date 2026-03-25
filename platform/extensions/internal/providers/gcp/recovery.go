package gcp

import (
	"context"
	"fmt"

	"github.com/runfabric/runfabric/platform/core/state/transactions"
	"github.com/runfabric/runfabric/platform/workflow/recovery"
)

type RecoveryHandler struct {
	journal *transactions.JournalFile
}

func NewRecoveryHandler(journal *transactions.JournalFile) recovery.Handler {
	return &RecoveryHandler{journal: journal}
}

func (h *RecoveryHandler) Rollback(_ context.Context, req recovery.Request) (*recovery.Result, error) {
	return &recovery.Result{
		Recovered: false,
		Mode:      string(recovery.ModeRollback),
		Status:    "manual_action_required",
		Message:   "gcp rollback requires manual cleanup or remove/deploy rerun",
		Metadata: map[string]string{
			"provider": "gcp-functions",
			"service":  req.Service,
			"stage":    req.Stage,
		},
	}, nil
}

func (h *RecoveryHandler) Resume(_ context.Context, req recovery.Request) (*recovery.Result, error) {
	return &recovery.Result{
		Recovered: false,
		Mode:      string(recovery.ModeResume),
		Status:    "manual_action_required",
		Message:   "gcp resume is not automatic; run deploy again after inspecting state",
		Metadata: map[string]string{
			"provider": "gcp-functions",
			"service":  req.Service,
			"stage":    req.Stage,
		},
	}, nil
}

func (h *RecoveryHandler) Inspect(_ context.Context, req recovery.Request) (*recovery.Result, error) {
	res := &recovery.Result{
		Recovered: false,
		Mode:      string(recovery.ModeInspect),
		Status:    "inspected",
		Message:   "gcp recovery inspect completed",
		Metadata: map[string]string{
			"provider": "gcp-functions",
			"service":  req.Service,
			"stage":    req.Stage,
		},
	}
	if h.journal != nil {
		res.Metadata["journalStatus"] = string(h.journal.Status)
		res.Metadata["checkpoints"] = fmt.Sprintf("%d", len(h.journal.Checkpoints))
	}
	return res, nil
}
