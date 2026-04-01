package secrets

import (
	"fmt"
	"strings"
)

// ValidateConfigSecretMap validates top-level config secrets map shape.
func ValidateConfigSecretMap(secretMap map[string]string) error {
	for k, v := range secretMap {
		if strings.TrimSpace(k) == "" {
			return fmt.Errorf("secrets contains empty key")
		}
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return fmt.Errorf("secrets.%s is empty", k)
		}
		if strings.HasPrefix(trimmed, "secret://") {
			ref := strings.TrimSpace(strings.TrimPrefix(trimmed, "secret://"))
			if ref == "" {
				return fmt.Errorf("secrets.%s has empty secret:// reference", k)
			}
		}
		if IsSecretManagerRef(trimmed) {
			if strings.TrimSpace(trimmed[strings.Index(trimmed, "://")+3:]) == "" {
				return fmt.Errorf("secrets.%s has empty secret manager reference", k)
			}
		}
	}
	return nil
}

// ValidateForStage applies stricter secret policy for production-like stages.
// Production stages must not use static literal secret values.
func ValidateForStage(secretMap map[string]string, stage string) error {
	if !isProductionStage(stage) {
		return nil
	}
	for key, value := range secretMap {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "${env:") ||
			strings.HasPrefix(trimmed, "${secret:") ||
			strings.HasPrefix(trimmed, "secret://") ||
			IsSecretManagerRef(trimmed) {
			continue
		}
		return fmt.Errorf(
			"secrets.%s uses a static literal in production stage %q; use ${env:VAR}, secret://KEY, or secret manager refs (%s)",
			key,
			strings.TrimSpace(stage),
			SecretManagerRefExamples(),
		)
	}
	return nil
}

func isProductionStage(stage string) bool {
	switch strings.ToLower(strings.TrimSpace(stage)) {
	case "prod", "production", "live":
		return true
	default:
		return false
	}
}
