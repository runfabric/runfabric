package providerpolicy

import (
	azurerouter "github.com/runfabric/runfabric/extensions/routers/azuretrafficmanager"
	cloudflarerouter "github.com/runfabric/runfabric/extensions/routers/cloudflare"
	ns1router "github.com/runfabric/runfabric/extensions/routers/ns1"
	"github.com/runfabric/runfabric/platform/extensions/providerpolicy/catalog"
)

// RouterCredentialVars declares the env vars the router subsystem reads and
// their per-request daemon headers (X-Router-*). Unlike providers these are
// subsystem-wide, not per-plugin: every router plugin reads its API token from
// RUNFABRIC_ROUTER_API_TOKEN (route53 is the exception — it authenticates via
// the AWS default chain and only needs zone/account identifiers).
//
// The daemon applies these to the process env for one operation only, exactly
// like provider credentials, so a multi-tenant daemon can sync DNS with the
// calling project's own router token instead of an ambient one.
func RouterCredentialVars() []catalog.CredentialVar {
	return []catalog.CredentialVar{
		{
			EnvKey:      "RUNFABRIC_ROUTER_API_TOKEN",
			Header:      "X-Router-Api-Token",
			Placeholder: "router-api-token",
		},
		{
			EnvKey:      "RUNFABRIC_ROUTER_ZONE_ID",
			Header:      "X-Router-Zone-Id",
			Placeholder: "zone-id",
		},
		{
			EnvKey:      "RUNFABRIC_ROUTER_ACCOUNT_ID",
			Header:      "X-Router-Account-Id",
			Placeholder: "account-id",
		},
	}
}

// RouterPluginCredentialVars returns each router plugin's OWN credential
// declarations, keyed by plugin ID and sourced from the router packages (the
// plugin is the single source of truth). All entries are env-only: the
// per-request header path is the shared subsystem group
// (RouterCredentialVars). Declarative Fallback entries point at same-cloud
// provider keys (e.g. cloudflare → CLOUDFLARE_API_TOKEN), so one set of
// provider credentials serves deploy and DNS sync unless router-specific
// values are configured. route53 declares none — AWS default chain.
func RouterPluginCredentialVars() map[string][]catalog.CredentialVar {
	return map[string][]catalog.CredentialVar{
		"cloudflare":            toCredentialVars(cloudflarerouter.CredentialVars),
		"ns1":                   toCredentialVars(ns1router.CredentialVars),
		"azure-traffic-manager": toCredentialVars(azurerouter.CredentialVars),
	}
}

// StateAWSCredentialVars declares the SHARED scoped AWS identity the
// AWS-backed state backends (s3, dynamodb) honor ahead of the AWS default
// chain — declared once at the subsystem level because per-request headers
// must be globally unique across credential groups. With these set, state can
// live in a different AWS account than the deploy target (X-Provider-Aws-*)
// and the aws-sm secret source (X-Secret-Aws-*). Resolution order lives in
// extensions/states/awsauth.
func StateAWSCredentialVars() []catalog.CredentialVar {
	return []catalog.CredentialVar{
		{
			EnvKey:      "RUNFABRIC_STATE_AWS_ACCESS_KEY_ID",
			Header:      "X-State-Aws-Access-Key-Id",
			Placeholder: "AKIA...",
		},
		{
			EnvKey:      "RUNFABRIC_STATE_AWS_SECRET_ACCESS_KEY",
			Header:      "X-State-Aws-Secret-Access-Key",
			Placeholder: "secret",
		},
		{
			EnvKey: "RUNFABRIC_STATE_AWS_SESSION_TOKEN",
			Header: "X-State-Aws-Session-Token",
		},
		{
			EnvKey:      "RUNFABRIC_STATE_AWS_REGION",
			Header:      "X-State-Aws-Region",
			Placeholder: "us-east-1",
		},
	}
}

// SecretManagerCredentialVars declares per-request credential vars for
// secret-manager backends, keyed by plugin ID.
//
// vault authenticates with a portable token. aws-secret-manager supports a
// SCOPED identity (RUNFABRIC_SM_AWS_*) that wins over the AWS default chain,
// so secrets can live in a different account than the deploy target
// (X-Provider-Aws-*) and the state store (X-State-Aws-*); unset, it falls
// back to the default chain (which the X-Provider-Aws-* group feeds).
// gcp-secret-manager and azure-key-vault-secret-manager authenticate via
// their cloud tooling's ambient identity and declare only env-only
// ADDRESSING vars (project / vault name); auth reuses the matching
// X-Provider-* groups.
func SecretManagerCredentialVars() map[string][]catalog.CredentialVar {
	return map[string][]catalog.CredentialVar{
		"gcp-secret-manager": {
			{EnvKey: "GCP_PROJECT_ID", Placeholder: "my-project"},
			{EnvKey: "GOOGLE_CLOUD_PROJECT"},
		},
		"azure-key-vault-secret-manager": {
			{EnvKey: "AZURE_KEY_VAULT_NAME", Required: true, Placeholder: "my-vault"},
		},
		"aws-secret-manager": {
			{
				EnvKey:      "RUNFABRIC_SM_AWS_ACCESS_KEY_ID",
				Header:      "X-Secret-Aws-Access-Key-Id",
				Placeholder: "AKIA...",
			},
			{
				EnvKey:      "RUNFABRIC_SM_AWS_SECRET_ACCESS_KEY",
				Header:      "X-Secret-Aws-Secret-Access-Key",
				Placeholder: "secret",
			},
			{
				EnvKey: "RUNFABRIC_SM_AWS_SESSION_TOKEN",
				Header: "X-Secret-Aws-Session-Token",
			},
			{
				EnvKey:      "RUNFABRIC_SM_AWS_REGION",
				Header:      "X-Secret-Aws-Region",
				Placeholder: "us-east-1",
			},
		},
		"vault-secret-manager": {
			{
				EnvKey:      "VAULT_ADDR",
				Header:      "X-Secret-Vault-Addr",
				Placeholder: "https://vault.example.com",
			},
			{
				EnvKey:      "VAULT_TOKEN",
				Header:      "X-Secret-Vault-Token",
				Required:    true,
				Placeholder: "hvs.example-token",
			},
			{
				EnvKey: "VAULT_NAMESPACE",
				Header: "X-Secret-Vault-Namespace",
			},
		},
	}
}
