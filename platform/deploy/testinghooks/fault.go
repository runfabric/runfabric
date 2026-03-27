package testinghooks

import (
	"os"

	deployexec "github.com/runfabric/runfabric/platform/deploy/exec"
)

func LoadFaultConfigFromEnv() deployexec.FaultConfig {
	return deployexec.FaultConfig{
		Enabled:         os.Getenv("RUNFABRIC_FAULT_ENABLED") == "1",
		FailBeforePhase: os.Getenv("RUNFABRIC_FAIL_BEFORE_PHASE"),
		FailAfterPhase:  os.Getenv("RUNFABRIC_FAIL_AFTER_PHASE"),
		FailOnResource:  os.Getenv("RUNFABRIC_FAIL_ON_RESOURCE"),
	}
}
