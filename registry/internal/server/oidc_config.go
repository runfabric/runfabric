package server

import "strings"

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func normalizeAudienceMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "exact":
		return "exact"
	case "includes":
		return "includes"
	case "skip":
		return "skip"
	default:
		return "exact"
	}
}

func parseRoleModes(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = "roles,realm_access.roles,resource_access.<client>.roles,scope"
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		mode := strings.TrimSpace(strings.ToLower(p))
		if mode == "" {
			continue
		}
		out = append(out, mode)
	}
	if len(out) == 0 {
		return []string{"roles", "scope"}
	}
	return out
}

func parseAllowedJWTAlgorithms(raw string) map[string]bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = "RS256,ES256"
	}
	out := map[string]bool{}
	for _, p := range strings.Split(raw, ",") {
		alg := strings.ToUpper(strings.TrimSpace(p))
		if alg == "" {
			continue
		}
		switch alg {
		case "RS256", "RS384", "RS512", "ES256", "ES384", "ES512":
			out[alg] = true
		}
	}
	if len(out) == 0 {
		out["RS256"] = true
	}
	return out
}
