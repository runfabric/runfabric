package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// FunctionConfigSignature returns a stable hash of the function config used for change detection.
// Includes runtime, handler, memory, timeout, architecture, environment, layers, tags.
// Providers may use a provider-specific signature (e.g. including resolved secrets); this is the neutral build-time signature.
func FunctionConfigSignature(fn FunctionConfig) (string, error) {
	payload := map[string]any{
		"runtime":      fn.Runtime,
		"handler":      fn.Handler,
		"memory":       fn.Memory,
		"timeout":      fn.Timeout,
		"architecture": fn.Architecture,
		"environment":  fn.Environment,
		"layers":       fn.Layers,
		"tags":         fn.Tags,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal config signature: %w", err)
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}
