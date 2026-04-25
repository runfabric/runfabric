package cluster

import (
	"os"

	"github.com/runfabric/runfabric/platform/observability/diagnostics"
)

func CheckBackends(service, stage string, locksBackend string, journalsBackend string, receiptsBackend string, lockErr, journalErr, receiptErr error) *diagnostics.HealthReport {
	report := &diagnostics.HealthReport{
		Service: service,
		Stage:   stage,
		Checks:  []diagnostics.CheckResult{},
	}

	report.Checks = append(report.Checks, diagnostics.CheckResult{
		Name:    "lock-backend",
		OK:      lockErr == nil,
		Backend: locksBackend,
		Message: errString(lockErr),
	})

	report.Checks = append(report.Checks, diagnostics.CheckResult{
		Name:    "journal-backend",
		OK:      journalErr == nil,
		Backend: journalsBackend,
		Message: errString(journalErr),
	})

	report.Checks = append(report.Checks, diagnostics.CheckResult{
		Name:    "receipt-backend",
		OK:      receiptErr == nil,
		Backend: receiptsBackend,
		Message: errString(receiptErr),
	})

	return report
}

func errString(err error) string {
	if err == nil {
		return "ok"
	}
	if os.IsNotExist(err) {
		return "not found"
	}
	return err.Error()
}
