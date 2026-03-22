package backends

import "github.com/runfabric/runfabric/platform/observability/diagnostics"

type Doctor interface {
	Doctor(service, stage string) diagnostics.CheckResult
}
