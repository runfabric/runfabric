package backends

import statetypes "github.com/runfabric/runfabric/internal/state/types"

type Doctor interface {
	Doctor(service, stage string) statetypes.CheckResult
}
