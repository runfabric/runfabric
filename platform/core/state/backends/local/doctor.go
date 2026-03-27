package local

import "github.com/runfabric/runfabric/platform/observability/diagnostics"

func (b *LockBackend) Doctor(service, stage string) diagnostics.CheckResult {
	return diagnostics.CheckResult{
		Name:    "lock-backend",
		OK:      true,
		Backend: b.Kind(),
		Message: "local backend ready",
	}
}
