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
	// Fallback is an env key consulted when EnvKey is unset — typically the
	// same-cloud provider credential (see plugin-sdk provider.ResolveVar).
	Fallback string
}

// ProviderScaffold is the provider-declared hint used by `runfabric init` to shape
// a generated project. The zero value means "use generic language defaults" — only
// the fields a provider genuinely diverges on need to be set (mirrors how a provider
// declares Credentials rather than init hardcoding them). Language×trigger handler
// bodies stay in platform/generator/application; this carries only provider deltas.
type ProviderScaffold struct {
	// Comment is the runfabric.yml header comment line (without a leading "# ").
	Comment string
	// Entry overrides functions[].entry (e.g. "worker.fetch"); empty = language default.
	Entry string
	// EntryFile is the handler file to write (e.g. "worker.js"); empty = src/handler.<ext>.
	EntryFile string
	// Sample overrides the handler body; empty = generator HandlerContent(lang,trigger).
	Sample string
	// RuntimeByLang overrides the runtime id per language (js/ts/node/python/go);
	// missing keys fall back to the generic map (nodejs20.x/python3.11/go1.x).
	RuntimeByLang map[string]string
}

// StateConfigLine is one backend.<Key>: <Value> line a state backend contributes
// to `runfabric init` output (converted from each backend's exported
// sdkprovider.ScaffoldConfigLine slice).
type StateConfigLine struct {
	Key   string
	Value string
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

	// Scaffold is the provider-declared `runfabric init` scaffolding hint.
	Scaffold ProviderScaffold
}

// ProviderPolicyEntry is the provider policy record consumed by the registry layer.
type ProviderPolicyEntry struct {
	Descriptor ProviderDescriptor
	Hooks      *inprocess.APIDispatchHooks
	Ops        inprocess.APIOps
}
