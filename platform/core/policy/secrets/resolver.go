package secrets

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

var secretPattern = regexp.MustCompile(`\$\{secret:([A-Za-z0-9_.-]+)\}`)

// ResolveString resolves ${secret:key} placeholders.
//
// Resolution order:
// 1) configSecrets[key]
// 2) process environment key
//
// Also supports `secret://NAME` values in configSecrets entries, where NAME is read from
// configSecrets[NAME] or the environment.
func ResolveString(input string, configSecrets map[string]string, lookup LookupFunc) (string, error) {
	if lookup == nil {
		lookup = os.LookupEnv
	}
	var firstErr error
	out := secretPattern.ReplaceAllStringFunc(input, func(match string) string {
		sub := secretPattern.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		value, err := resolveSecretKey(sub[1], configSecrets, lookup, 0)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			return match
		}
		return value
	})
	if firstErr != nil {
		return "", firstErr
	}
	return out, nil
}

func resolveSecretKey(key string, configSecrets map[string]string, lookup LookupFunc, depth int) (string, error) {
	if depth > 8 {
		return "", fmt.Errorf("secret resolution exceeded max depth for %q", key)
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return "", fmt.Errorf("secret key is empty")
	}

	if configSecrets != nil {
		if v, ok := configSecrets[key]; ok {
			v = strings.TrimSpace(v)
			if strings.HasPrefix(v, "secret://") {
				ref := strings.TrimSpace(strings.TrimPrefix(v, "secret://"))
				if ref == "" {
					return "", fmt.Errorf("secret %q references empty secret:// ref", key)
				}
				return resolveSecretKey(ref, configSecrets, lookup, depth+1)
			}
			return v, nil
		}
	}
	if v, ok := lookup(key); ok && strings.TrimSpace(v) != "" {
		return v, nil
	}
	return "", fmt.Errorf("config references ${secret:%s} but %s is not set", key, key)
}
