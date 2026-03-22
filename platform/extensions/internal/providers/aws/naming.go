package aws

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/runfabric/runfabric/platform/core/model/config"
)

var invalidNameChars = regexp.MustCompile(`[^a-zA-Z0-9-_]`)

func functionName(cfg *config.Config, stage, fn string) string {
	raw := fmt.Sprintf("%s-%s-%s", cfg.Service, stage, fn)
	return sanitizeName(raw)
}

func roleName(cfg *config.Config, stage, fn string) string {
	raw := fmt.Sprintf("%s-%s-%s-role", cfg.Service, stage, fn)
	return sanitizeName(raw)
}

func httpAPIName(cfg *config.Config, stage string) string {
	raw := fmt.Sprintf("%s-%s-http", cfg.Service, stage)
	return sanitizeName(raw)
}

func lambdaPermissionStatementID(cfg *config.Config, stage, fn, method, path string) string {
	raw := fmt.Sprintf("%s-%s-%s-%s-%s-apigw", cfg.Service, stage, fn, method, path)
	return sanitizeName(raw)
}

func sanitizeName(s string) string {
	out := invalidNameChars.ReplaceAllString(s, "-")
	out = strings.Trim(out, "-")
	if len(out) > 64 {
		out = out[:64]
	}
	return out
}
