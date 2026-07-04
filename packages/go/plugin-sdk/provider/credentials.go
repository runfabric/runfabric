package provider

// CredentialVar declares one credential environment variable a provider reads.
// Providers publish their credential surface as data so every host derives it
// from the same declaration instead of hardcoding lists: the CLI doctor checks
// Required vars, project scaffolding writes .env.example entries, and the
// daemon maps per-request X-Provider-* headers onto the process env.
type CredentialVar struct {
	// EnvKey is the process environment variable the provider reads.
	EnvKey string
	// Header is the daemon's per-request header carrying this value
	// (X-Provider-*). Empty means env-only: the value cannot be supplied
	// per request (e.g. file-based credentials like KUBECONFIG).
	Header string
	// Required marks variables the provider hard-fails without at deploy time.
	Required bool
	// Mirror is an env key kept in lockstep with EnvKey — set to the same
	// value and cleared together — for SDKs that accept either spelling
	// (e.g. AWS_REGION → AWS_DEFAULT_REGION, GCP_PROJECT → GCP_PROJECT_ID).
	Mirror string
	// Placeholder is an example value for generated .env scaffolding.
	Placeholder string
}
