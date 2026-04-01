package secretmanagers

import sdkrouter "github.com/runfabric/runfabric/plugin-sdk/go/router"

// BuiltinSecretManagerManifests returns secret-manager metadata entries exposed
// in built-in extension catalogs.
func BuiltinSecretManagerManifests() []sdkrouter.PluginMeta {
	return []sdkrouter.PluginMeta{
		{
			ID:          "aws-secret-manager",
			Name:        "AWS Secret Manager",
			Description: "Resolves aws-sm:// secret references from AWS Secrets Manager",
		},
		{
			ID:          "gcp-secret-manager",
			Name:        "GCP Secret Manager",
			Description: "Resolves gcp-sm:// secret references via Google Secret Manager",
		},
		{
			ID:          "azure-key-vault-secret-manager",
			Name:        "Azure Key Vault Secret Manager",
			Description: "Resolves azure-kv:// secret references via Azure Key Vault",
		},
		{
			ID:          "vault-secret-manager",
			Name:        "Vault Secret Manager",
			Description: "Resolves vault:// secret references via HashiCorp Vault",
		},
	}
}
