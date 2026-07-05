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
	// Fallback is an env key consulted when EnvKey is unset — typically the
	// same-cloud provider credential, so one set of provider creds serves
	// dependent extensions (e.g. the cloudflare router's
	// RUNFABRIC_ROUTER_API_TOKEN falls back to CLOUDFLARE_API_TOKEN).
	// Resolution goes through ResolveVar; explicit values always win.
	Fallback string
}

// ResolveVar resolves the credential named envKey from vars: the env var
// itself first, then the declared Fallback. Returns "" when neither is set or
// envKey is not declared.
func ResolveVar(vars []CredentialVar, envKey string) string {
	for _, v := range vars {
		if v.EnvKey != envKey {
			continue
		}
		if value := Env(v.EnvKey); value != "" {
			return value
		}
		if v.Fallback != "" {
			return Env(v.Fallback)
		}
		return ""
	}
	return ""
}
