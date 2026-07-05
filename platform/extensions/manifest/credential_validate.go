package manifests

import (
	"fmt"
	"regexp"
	"strings"
)

// Credential declaration shape rules, shared by external plugin.yaml loading
// and the built-in declaration tests, so every extension kind's credentials
// obey the same contract regardless of origin.
var (
	// envKeyPattern: standard uppercase env var names (AWS_REGION,
	// RUNFABRIC_ROUTER_API_TOKEN, ...).
	envKeyPattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)
	// headerPattern: per-request daemon headers (X-Provider-Aws-Region,
	// X-State-Postgres-Url, ...). Must start with "X-".
	headerPattern = regexp.MustCompile(`^X-[A-Za-z0-9]+(-[A-Za-z0-9]+)*$`)
)

// ValidateCredentialSpecs checks a plugin's credential declarations:
// well-formed env keys, well-formed X-* headers, mirrors that are real env
// keys distinct from their source, and no duplicate env keys or headers
// within the plugin. Returns the first problem found.
func ValidateCredentialSpecs(creds []CredentialSpec) error {
	seenEnv := map[string]bool{}
	seenHeader := map[string]bool{}
	for i, c := range creds {
		envKey := strings.TrimSpace(c.EnvKey)
		if envKey == "" {
			return fmt.Errorf("credentials[%d]: envKey is required", i)
		}
		if !envKeyPattern.MatchString(envKey) {
			return fmt.Errorf("credentials[%d]: envKey %q must match %s", i, envKey, envKeyPattern)
		}
		if seenEnv[envKey] {
			return fmt.Errorf("credentials[%d]: duplicate envKey %q", i, envKey)
		}
		seenEnv[envKey] = true

		if header := strings.TrimSpace(c.Header); header != "" {
			if !headerPattern.MatchString(header) {
				return fmt.Errorf("credentials[%d]: header %q must match %s", i, header, headerPattern)
			}
			if seenHeader[header] {
				return fmt.Errorf("credentials[%d]: duplicate header %q", i, header)
			}
			seenHeader[header] = true
		}

		if mirror := strings.TrimSpace(c.Mirror); mirror != "" {
			if !envKeyPattern.MatchString(mirror) {
				return fmt.Errorf("credentials[%d]: mirror %q must match %s", i, mirror, envKeyPattern)
			}
			if mirror == envKey {
				return fmt.Errorf("credentials[%d]: mirror %q must differ from envKey", i, mirror)
			}
		}

		if fallback := strings.TrimSpace(c.Fallback); fallback != "" {
			// Well-formedness only: the fallback typically names a same-cloud
			// provider key the plugin does NOT declare itself.
			if !envKeyPattern.MatchString(fallback) {
				return fmt.Errorf("credentials[%d]: fallback %q must match %s", i, fallback, envKeyPattern)
			}
			if fallback == envKey {
				return fmt.Errorf("credentials[%d]: fallback %q must differ from envKey", i, fallback)
			}
		}
	}
	// A mirror must not collide with another declared envKey — the two would
	// fight over the same variable during the daemon's group clear/apply.
	for i, c := range creds {
		if mirror := strings.TrimSpace(c.Mirror); mirror != "" && seenEnv[mirror] {
			return fmt.Errorf("credentials[%d]: mirror %q collides with a declared envKey", i, mirror)
		}
	}
	return nil
}
