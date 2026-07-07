package provider

// Scaffold declares the provider-specific parts of `runfabric init` project
// scaffolding as data, so the CLI derives them from the provider instead of
// hardcoding a per-provider switch. Language×trigger handler bodies stay generic
// (platform/generator/application); this carries only the deltas a provider needs.
// The zero value means "use generic language defaults".
type Scaffold struct {
	// Comment is the runfabric.yml header comment line (without a leading "# ").
	Comment string
	// Entry overrides functions[].entry (e.g. "worker.fetch"); empty = language default.
	Entry string
	// EntryFile is the handler file to write (e.g. "worker.js"); empty = src/handler.<ext>.
	EntryFile string
	// Sample overrides the generated handler body; empty = the generic
	// language×trigger sample.
	Sample string
	// RuntimeByLang overrides the runtime id per language key (js/ts/node/python/go);
	// missing keys fall back to the generic map (nodejs20.x/python3.11/go1.x).
	RuntimeByLang map[string]string
}

// ScaffoldConfigLine is one backend.<Key>: <Value> line a state backend
// contributes to `runfabric init` (the state-side scaffold shape). State backends
// reuse this SDK type the same way they reuse CredentialVar.
type ScaffoldConfigLine struct {
	// Key is the config.BackendConfig field (yaml tag), e.g. "s3Bucket".
	Key string
	// Value is the line value: a literal (runfabric/dev) or env ref (${env:...}).
	Value string
}
