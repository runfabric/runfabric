package config

import (
	"fmt"
	"strings"
)

// AutoInstallExtensions reports whether runfabric.yml requests auto-install of missing extensions.
//
// Supported keys:
// - extensions.autoInstallExtensions: true|false (preferred)
// - extensions.autoInstall: true|false (alias)
func AutoInstallExtensions(cfg *Config) bool {
	if cfg == nil || cfg.Extensions == nil {
		return false
	}
	for _, k := range []string{"autoInstallExtensions", "autoInstall"} {
		v, ok := cfg.Extensions[k]
		if !ok {
			continue
		}
		switch t := v.(type) {
		case bool:
			return t
		case string:
			s := strings.ToLower(strings.TrimSpace(t))
			return s == "1" || s == "true" || s == "yes"
		}
	}
	return false
}

// ExtensionString returns an extensions.<key> value as a string.
// Supported value types: string, fmt.Stringer, or any scalar value formatted via fmt.Sprint.
func ExtensionString(cfg *Config, key string) string {
	if cfg == nil || cfg.Extensions == nil {
		return ""
	}
	v, ok := cfg.Extensions[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case fmt.Stringer:
		return strings.TrimSpace(t.String())
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}
