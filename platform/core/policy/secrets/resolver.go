package secrets

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
)

var secretPattern = regexp.MustCompile(`\$\{secret:([A-Za-z0-9_.-]+)\}`)

var (
	referenceResolverMu sync.RWMutex
	referenceResolver   ReferenceResolver

	secretManagerSchemeMu sync.RWMutex
	secretManagerSchemes  = map[string]struct{}{}
)

// ReferenceResolver resolves secret-manager references (for example
// aws-sm://..., gcp-sm://..., azure-kv://..., vault://...) into concrete secret values.
type ReferenceResolver func(ref string) (string, error)

// SetReferenceResolver sets the process-wide secret-manager reference resolver and
// returns a restore function that resets the previous resolver.
func SetReferenceResolver(resolver ReferenceResolver) func() {
	referenceResolverMu.Lock()
	prev := referenceResolver
	referenceResolver = resolver
	referenceResolverMu.Unlock()
	return func() {
		referenceResolverMu.Lock()
		referenceResolver = prev
		referenceResolverMu.Unlock()
	}
}

// SetSecretManagerRefSchemes sets allowed secret-manager reference URI schemes
// (without ://) used by IsSecretManagerRef. It returns a restore function.
func SetSecretManagerRefSchemes(schemes []string) func() {
	normalized := make(map[string]struct{}, len(schemes))
	for _, raw := range schemes {
		if scheme := normalizeSecretManagerRefScheme(raw); scheme != "" {
			normalized[scheme] = struct{}{}
		}
	}

	secretManagerSchemeMu.Lock()
	prev := cloneSchemeSet(secretManagerSchemes)
	secretManagerSchemes = normalized
	secretManagerSchemeMu.Unlock()

	return func() {
		secretManagerSchemeMu.Lock()
		secretManagerSchemes = prev
		secretManagerSchemeMu.Unlock()
	}
}

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
			return resolveSecretManagerRefValue(v)
		}
	}
	if v, ok := lookup(key); ok && strings.TrimSpace(v) != "" {
		return resolveSecretManagerRefValue(v)
	}
	return "", fmt.Errorf("config references ${secret:%s} but %s is not set", key, key)
}

func resolveSecretManagerRefValue(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", nil
	}
	if !IsSecretManagerRef(value) {
		return value, nil
	}
	referenceResolverMu.RLock()
	resolver := referenceResolver
	referenceResolverMu.RUnlock()
	if resolver == nil {
		return "", fmt.Errorf(
			"secret manager reference %q requires extensions.secretManagerPlugin to be installed and configured",
			value,
		)
	}
	resolved, err := resolver(value)
	if err != nil {
		return "", err
	}
	resolved = strings.TrimSpace(resolved)
	if resolved == "" {
		return "", fmt.Errorf("secret manager reference %q resolved to empty value", value)
	}
	return resolved, nil
}

// IsSecretManagerRef reports whether value uses a secret-manager reference
// scheme resolved through a secret-manager plugin.
func IsSecretManagerRef(value string) bool {
	scheme := extractRefScheme(value)
	if scheme == "" {
		return false
	}
	secretManagerSchemeMu.RLock()
	_, ok := secretManagerSchemes[scheme]
	secretManagerSchemeMu.RUnlock()
	return ok
}

// SecretManagerRefExamples returns configured secret-manager ref examples
// suitable for help/error messages, e.g. "vault://, aws-sm://".
func SecretManagerRefExamples() string {
	secretManagerSchemeMu.RLock()
	if len(secretManagerSchemes) == 0 {
		secretManagerSchemeMu.RUnlock()
		return "<scheme>://"
	}
	list := make([]string, 0, len(secretManagerSchemes))
	for scheme := range secretManagerSchemes {
		list = append(list, scheme)
	}
	secretManagerSchemeMu.RUnlock()

	sort.Strings(list)
	for i, item := range list {
		list[i] = item + "://"
	}
	return strings.Join(list, ", ")
}

func extractRefScheme(value string) string {
	trimmed := strings.TrimSpace(value)
	sep := strings.Index(trimmed, "://")
	if sep <= 0 {
		return ""
	}
	return normalizeSecretManagerRefScheme(trimmed[:sep])
}

func normalizeSecretManagerRefScheme(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	s = strings.TrimSuffix(s, "://")
	return strings.TrimSpace(s)
}

func cloneSchemeSet(src map[string]struct{}) map[string]struct{} {
	if len(src) == 0 {
		return map[string]struct{}{}
	}
	out := make(map[string]struct{}, len(src))
	for k := range src {
		out[k] = struct{}{}
	}
	return out
}
