package aws

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/runfabric/runfabric/internal/config"
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

func hashMap(m map[string]string) string {
	if len(m) == 0 {
		return ""
	}
	b, _ := json.Marshal(m)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func hashSlice(v []string) string {
	if len(v) == 0 {
		return ""
	}
	b, _ := json.Marshal(v)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
