package app

import (
	"fmt"
	"strings"
)

func validateServiceScope(configService, requestedService string) error {
	requested := strings.TrimSpace(requestedService)
	if requested == "" {
		return nil
	}
	configured := strings.TrimSpace(configService)
	if configured == "" {
		return fmt.Errorf("service scope requested (%q) but config service is empty", requested)
	}
	if requested != configured {
		return fmt.Errorf("service scope mismatch: requested %q but config service is %q", requested, configured)
	}
	return nil
}
