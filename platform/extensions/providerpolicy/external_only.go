package providerpolicy

import "strings"

// ExternalOnlyProviders is the runtime toggle surface for built-in providers.
//
// Set a provider ID to true to force external-only loading for that provider.
// This lets you flip provider mode by editing only this file.
//
// Example:
//
//	"aws-lambda": true,
//
// Valid IDs:
//
//	aws-lambda, gcp-functions, azure-functions, alibaba-fc,
//	cloudflare-workers, digitalocean-functions, fly-machines,
//	ibm-openwhisk, kubernetes, netlify, vercel
var ExternalOnlyProviders = map[string]bool{}

// ExternalOnlyRouters marks built-in router IDs that must not use in-process built-ins.
//
// Valid IDs:
//
//	cloudflare, route53, ns1, azure-traffic-manager
var ExternalOnlyRouters = map[string]bool{}

// ExternalOnlyRuntimes marks built-in runtime IDs that must not use in-process built-ins.
//
// Valid IDs:
//
//	nodejs, python
var ExternalOnlyRuntimes = map[string]bool{}

// ExternalOnlySimulators marks built-in simulator IDs that must not use in-process built-ins.
//
// Valid IDs:
//
//	local
var ExternalOnlySimulators = map[string]bool{}

// ExternalOnlyStates marks built-in state manifest IDs hidden from built-in catalog registration.
//
// Valid IDs:
//
//	local, sqlite, postgres, dynamodb
var ExternalOnlyStates = map[string]bool{}

// ExternalOnlySecretManagers marks built-in secret-manager manifest IDs hidden from built-in catalog registration.
//
// Valid IDs:
//
//	aws-secret-manager, gcp-secret-manager, azure-key-vault-secret-manager, vault-secret-manager
var ExternalOnlySecretManagers = map[string]bool{}

func isExternalOnlyProvider(id string) bool {
	return ExternalOnlyProviders[strings.TrimSpace(id)]
}

// IsExternalOnlyProvider reports whether a provider ID is forced to external-only mode.
func IsExternalOnlyProvider(id string) bool {
	return isExternalOnlyProvider(id)
}

func isExternalOnlyRouter(id string) bool {
	return ExternalOnlyRouters[strings.TrimSpace(id)]
}

// IsExternalOnlyRouter reports whether a router ID is forced to external-only mode.
func IsExternalOnlyRouter(id string) bool {
	return isExternalOnlyRouter(id)
}

func isExternalOnlyRuntime(id string) bool {
	return ExternalOnlyRuntimes[strings.TrimSpace(id)]
}

// IsExternalOnlyRuntime reports whether a runtime ID is forced to external-only mode.
func IsExternalOnlyRuntime(id string) bool {
	return isExternalOnlyRuntime(id)
}

func isExternalOnlySimulator(id string) bool {
	return ExternalOnlySimulators[strings.TrimSpace(id)]
}

// IsExternalOnlySimulator reports whether a simulator ID is forced to external-only mode.
func IsExternalOnlySimulator(id string) bool {
	return isExternalOnlySimulator(id)
}

func isExternalOnlyState(id string) bool {
	return ExternalOnlyStates[strings.TrimSpace(id)]
}

// IsExternalOnlyState reports whether a state backend ID is forced to external-only mode.
func IsExternalOnlyState(id string) bool {
	return isExternalOnlyState(id)
}

func isExternalOnlySecretManager(id string) bool {
	return ExternalOnlySecretManagers[strings.TrimSpace(id)]
}

// IsExternalOnlySecretManager reports whether a secret-manager ID is forced to external-only mode.
func IsExternalOnlySecretManager(id string) bool {
	return isExternalOnlySecretManager(id)
}
