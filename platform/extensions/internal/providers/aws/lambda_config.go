package aws

import (
	"github.com/runfabric/runfabric/platform/core/model/config"
)

func environmentMap(fn config.FunctionConfig) map[string]string {
	out := map[string]string{}
	for k, v := range fn.Environment {
		out[k] = v
	}
	for k, v := range resolvedSecrets(fn) {
		out[k] = v
	}
	return out
}

func resolvedSecrets(fn config.FunctionConfig) map[string]string {
	// Secrets are passed through as-is. SSM/Secrets Manager resolution can be added per provider standard.
	out := map[string]string{}
	for k, v := range fn.Secrets {
		out[k] = v
	}
	return out
}
