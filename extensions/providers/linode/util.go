package main

import (
	"fmt"
	"strings"
)

func asString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		return ""
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstNonNil(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func joinRoot(root, value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "/") {
		return trimmed
	}
	if strings.TrimSpace(root) == "" {
		return trimmed
	}
	return pathJoin(root, trimmed)
}

func pathJoin(parts ...string) string {
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		filtered = append(filtered, strings.Trim(part, "/"))
	}
	if len(filtered) == 0 {
		return ""
	}
	if strings.HasPrefix(strings.TrimSpace(parts[0]), "/") {
		return "/" + strings.Join(filtered, "/")
	}
	return strings.Join(filtered, "/")
}

func pathDir(value string) string {
	trimmed := strings.TrimSpace(value)
	index := strings.LastIndex(trimmed, "/")
	if index < 0 {
		return ""
	}
	return trimmed[:index]
}

func pathBase(value string) string {
	trimmed := strings.TrimSpace(value)
	index := strings.LastIndex(trimmed, "/")
	if index < 0 {
		return trimmed
	}
	return trimmed[index+1:]
}
