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
		if strings.TrimSpace(v) == "" {
			return fmt.Errorf("secrets.%s is empty", k)
		}
		if strings.HasPrefix(strings.TrimSpace(v), "secret://") {
			ref := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(v), "secret://"))
			if ref == "" {
				return fmt.Errorf("secrets.%s has empty secret:// reference", k)
			}
		}
	}
	return nil
}
