package common

// Env returns environment variables for the given runtime and stage.
func Env(runtime Runtime, stage string) map[string]string {
	return map[string]string{"RUNFABRIC_STAGE": stage}
}
