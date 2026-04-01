package local

import statetypes "github.com/runfabric/runfabric/extensions/types"

func (b *LockBackend) Doctor(service, stage string) statetypes.CheckResult {
	return statetypes.CheckResult{
		Name:    "lock-backend",
		OK:      true,
		Backend: b.Kind(),
		Message: "local backend ready",
	}
}
