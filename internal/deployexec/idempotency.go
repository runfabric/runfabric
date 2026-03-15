package deployexec

import "github.com/runfabric/runfabric/internal/transactions"

func RecordOnce(journal *transactions.Journal, op transactions.Operation) error {
	if journal == nil || journal.File() == nil {
		return nil
	}

	for _, existing := range journal.File().Operations {
		if existing.Type == op.Type && existing.Resource == op.Resource {
			return nil
		}
	}

	return journal.Record(op)
}
