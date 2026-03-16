package transactions

import "context"

type Rollbacker interface {
	Rollback(ctx context.Context, op Operation) error
}

func ExecuteRollback(ctx context.Context, rb Rollbacker, journal *Journal) []error {
	if journal == nil {
		return nil
	}

	var errs []error
	for _, op := range journal.Reverse() {
		if err := rb.Rollback(ctx, op); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}
