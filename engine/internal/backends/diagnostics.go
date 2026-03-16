package backends

import "github.com/runfabric/runfabric/engine/internal/diagnostics"

type Doctor interface {
	Doctor(service, stage string) diagnostics.CheckResult
}
