package aws

import (
	"context"
	"fmt"

	appErrs "github.com/runfabric/runfabric/engine/internal/errors"
	"github.com/runfabric/runfabric/engine/internal/recovery"
	"github.com/runfabric/runfabric/engine/internal/transactions"
)

type RecoveryHandler struct {
	journal *transactions.JournalFile
}

func NewRecoveryHandler(journal *transactions.JournalFile) *RecoveryHandler {
	return &RecoveryHandler{journal: journal}
}

func (h *RecoveryHandler) Rollback(ctx context.Context, req recovery.Request) (*recovery.Result, error) {
	journalBackend := transactions.NewFileBackend(req.Root)

	clients, err := loadClients(ctx, req.Region)
	if err != nil {
		return nil, appErrs.Wrap(appErrs.CodeDeployFailed, "load aws clients for recovery failed", err)
	}

	journal := transactions.NewJournal(req.Service, req.Stage, h.journal.Operation, journalBackend)
	journal.File().Status = h.journal.Status
	journal.File().Operations = h.journal.Operations
	journal.File().Checkpoints = h.journal.Checkpoints
	journal.File().StartedAt = h.journal.StartedAt
	journal.File().UpdatedAt = h.journal.UpdatedAt

	if err := journal.MarkRollingBack(); err != nil {
		return nil, appErrs.Wrap(appErrs.CodeDeployFailed, "mark journal rolling_back failed", err)
	}

	rb := newAWSRollbacker(clients)
	errs := transactions.ExecuteRollback(ctx, rb, journal)

	if len(errs) > 0 {
		return &recovery.Result{
			Recovered: false,
			Mode:      string(recovery.ModeRollback),
			Status:    "partial_failure",
			Errors:    stringifyErrors(errs),
		}, nil
	}

	if err := journal.MarkRolledBack(); err != nil {
		return nil, appErrs.Wrap(appErrs.CodeDeployFailed, "mark journal rolled_back failed", err)
	}

	if err := journal.Delete(); err != nil {
		return nil, appErrs.Wrap(appErrs.CodeDeployFailed, "delete recovery journal failed", err)
	}

	return &recovery.Result{
		Recovered: true,
		Mode:      string(recovery.ModeRollback),
		Status:    "rolled_back",
		Metadata: map[string]string{
			"service": req.Service,
			"stage":   req.Stage,
		},
	}, nil
}

func (h *RecoveryHandler) Resume(ctx context.Context, req recovery.Request) (*recovery.Result, error) {
	// journalBackend := transactions.NewFileBackend(req.Root)

	// journal := transactions.NewJournal(req.Service, req.Stage, h.journal.Operation, journalBackend)
	// journal.File().Status = h.journal.Status
	// journal.File().Operations = h.journal.Operations
	// journal.File().Checkpoints = h.journal.Checkpoints
	// journal.File().StartedAt = h.journal.StartedAt
	// journal.File().UpdatedAt = h.journal.UpdatedAt

	return &recovery.Result{
		Recovered: false,
		Mode:      string(recovery.ModeResume),
		Status:    "handoff_to_deploy_resume",
		Message:   "resume is executed by the deploy resume engine via app recovery flow",
		Metadata: map[string]string{
			"journalStatus": string(h.journal.Status),
			"checkpoints":   fmt.Sprintf("%d", len(h.journal.Checkpoints)),
		},
	}, nil
}

func (h *RecoveryHandler) Inspect(ctx context.Context, req recovery.Request) (*recovery.Result, error) {
	return &recovery.Result{
		Recovered: false,
		Mode:      string(recovery.ModeInspect),
		Status:    string(h.journal.Status),
		Message:   "inspection only",
		Metadata: map[string]string{
			"service":    req.Service,
			"stage":      req.Stage,
			"operation":  h.journal.Operation,
			"startedAt":  h.journal.StartedAt,
			"updatedAt":  h.journal.UpdatedAt,
			"operations": fmt.Sprintf("%d", len(h.journal.Operations)),
		},
	}, nil
}

func recoverIfNeeded(ctx context.Context, root, service, stage string, clients *AWSClients) error {
	jb := transactions.NewFileBackend(root)

	jf, err := jb.Load(service, stage)
	if err != nil {
		return nil
	}

	switch jf.Status {
	case transactions.StatusCompleted, transactions.StatusRolledBack:
		return jb.Delete(service, stage)
	case transactions.StatusActive, transactions.StatusRollingBack:
		return fmt.Errorf("unfinished transaction journal exists for service=%s stage=%s status=%s; recover or clean it before continuing", service, stage, jf.Status)
	default:
		return nil
	}
}

func stringifyErrors(errs []error) []string {
	out := make([]string, 0, len(errs))
	for _, err := range errs {
		out = append(out, fmt.Sprintf("%v", err))
	}
	return out
}
