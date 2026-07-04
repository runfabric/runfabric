package catalog

import (
	"github.com/runfabric/runfabric/platform/extensions/inprocess"
)

// CredentialVar is one credential environment variable a provider declares
// (mirrors the plugin-sdk CredentialVar; converted at the providerpolicy gate
// because platform code must not import the plugin SDK).
type CredentialVar struct {
	// EnvKey is the process environment variable the provider reads.
	EnvKey string
	// Header is the daemon's per-request header (X-Provider-*); empty = env-only.
	Header string
	// Required marks variables the provider hard-fails without at deploy time.
	Required bool
	// Mirror is an env key kept in lockstep with EnvKey (set/cleared together).
	Mirror string
	// Placeholder is an example value for generated .env scaffolding.
	Placeholder string
}

// ProviderDescriptor defines how a provider should be surfaced by the extension system.
type ProviderDescriptor struct {
	ID string

	Name        string
	Description string

	BuiltinImplementation  bool
	ExcludeFromAPIDispatch bool
	IncludeBuiltinManifest bool

	// Credentials is the provider-declared credential env surface.
	Credentials []CredentialVar
}

// ProviderPolicyEntry is the provider policy record consumed by the registry layer.
type ProviderPolicyEntry struct {
	Descriptor ProviderDescriptor
	Hooks      *inprocess.APIDispatchHooks
	Ops        inprocess.APIOps
}
