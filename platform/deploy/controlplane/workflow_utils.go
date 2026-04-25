package controlplane

import (
	"strconv"
	"strings"
)

func asInputString(obj map[string]any, key string) string {
	if obj == nil {
		return ""
	}
	s, _ := obj[key].(string)
	return strings.TrimSpace(s)
}

func asFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case int32:
		return float64(n), true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(n), 64)
		if err == nil {
			return f, true
		}
	}
	return 0, false
}
