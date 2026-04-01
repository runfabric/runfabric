package states

import "strings"

// ExpandLookupAliases normalizes additional lookup aliases for state backends.
// State backend resolution now only uses "local" and does not add aliases.
func ExpandLookupAliases(keys map[string]struct{}) {
	_ = keys
}

// BackendKindFromPlugin derives backend.kind from capabilities first, then plugin ID.
func BackendKindFromPlugin(pluginID string, capabilities []string) (string, bool) {
	if kind, ok := BackendKindFromCapabilities(capabilities); ok {
		return kind, true
	}
	return BackendKindFromPluginID(pluginID)
}

// BackendKindFromCapabilities extracts backend.kind from plugin capabilities.
func BackendKindFromCapabilities(capabilities []string) (string, bool) {
	for _, raw := range capabilities {
		if kind, ok := BackendKindFromCapability(raw); ok {
			return kind, true
		}
	}
	return "", false
}

// BackendKindFromCapability parses a single capability token.
func BackendKindFromCapability(raw string) (string, bool) {
	capability := strings.ToLower(strings.TrimSpace(raw))
	if capability == "" {
		return "", false
	}
	switch {
	case strings.HasPrefix(capability, "backend:"):
		capability = strings.TrimSpace(strings.TrimPrefix(capability, "backend:"))
	case strings.HasPrefix(capability, "state:"):
		capability = strings.TrimSpace(strings.TrimPrefix(capability, "state:"))
	}
	return NormalizeBackendKindToken(capability)
}

// BackendKindFromPluginID extracts backend.kind from state plugin ID/name.
func BackendKindFromPluginID(pluginID string) (string, bool) {
	return NormalizeBackendKindToken(pluginID)
}

// NormalizeBackendKindToken normalizes a state token into a backend.kind value.
func NormalizeBackendKindToken(raw string) (string, bool) {
	token := normalizedToken(raw)
	if token == "" {
		return "", false
	}
	token = strings.Trim(token, "-")
	if token == "" {
		return "", false
	}
	if strings.Contains(token, "-") {
		parts := strings.Split(token, "-")
		token = strings.TrimSpace(parts[len(parts)-1])
	}
	if token == "" {
		return "", false
	}
	return token, true
}

func normalizedToken(raw string) string {
	v := strings.ToLower(strings.TrimSpace(raw))
	if v == "" {
		return ""
	}
	replaced := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return '-'
	}, v)
	replaced = strings.Trim(replaced, "-")
	for strings.Contains(replaced, "--") {
		replaced = strings.ReplaceAll(replaced, "--", "-")
	}
	return strings.TrimSpace(replaced)
}
