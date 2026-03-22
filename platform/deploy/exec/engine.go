package exec

import (
	"context"

	"github.com/runfabric/runfabric/platform/core/state/transactions"
)

type Engine struct {
	Phases  []Phase
	Journal *transactions.Journal
}

func (e *Engine) Run(ctx context.Context, execCtx *Context) error {
	completed := completedCheckpoints(e.Journal)

	for _, phase := range e.Phases {
		if completed[phase.Name()] {
			continue
		}

		if err := execCtx.Faults.CheckBefore(phase.Name()); err != nil {
			return err
		}

		if err := e.Journal.Checkpoint(phase.Name(), "in_progress"); err != nil {
			return err
		}

		if err := phase.Run(ctx, execCtx); err != nil {
			return err
		}

		if err := e.Journal.Checkpoint(phase.Name(), "done"); err != nil {
			return err
		}

		if err := execCtx.Faults.CheckAfter(phase.Name()); err != nil {
			return err
		}
	}

	return nil
}

func completedCheckpoints(j *transactions.Journal) map[string]bool {
	out := map[string]bool{}
	if j == nil || j.File() == nil {
		return out
	}

	for _, cp := range j.File().Checkpoints {
		if cp.Status == "done" {
			out[cp.Name] = true
		}
	}

	return out
}
