package secrets

import (
	"os"
	"strings"
)

// RequiredProviderEnvVars returns the list of required environment variable names for the given provider.
// Aligned with docs/CREDENTIALS.md. Provider name should be the config provider name (e.g. aws-lambda, gcp-functions).
func RequiredProviderEnvVars(provider string) []string {
	switch strings.TrimSpace(strings.ToLower(provider)) {
	case "aws-lambda":
		return []string{"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_REGION"}
	case "gcp-functions":
		return []string{"GCP_PROJECT_ID", "GCP_SERVICE_ACCOUNT_KEY"}
	case "azure-functions":
		return []string{"AZURE_TENANT_ID", "AZURE_CLIENT_ID", "AZURE_CLIENT_SECRET", "AZURE_SUBSCRIPTION_ID", "AZURE_RESOURCE_GROUP"}
	case "kubernetes":
		return []string{"KUBECONFIG"}
	case "cloudflare-workers":
		return []string{"CLOUDFLARE_API_TOKEN", "CLOUDFLARE_ACCOUNT_ID"}
	case "vercel":
		return []string{"VERCEL_TOKEN"}
	case "netlify":
		return []string{"NETLIFY_AUTH_TOKEN", "NETLIFY_SITE_ID"}
	case "alibaba-fc":
		return []string{"ALICLOUD_ACCESS_KEY_ID", "ALICLOUD_ACCESS_KEY_SECRET", "ALICLOUD_REGION"}
	case "digitalocean-functions":
		return []string{"DIGITALOCEAN_ACCESS_TOKEN"}
	case "fly-machines":
		return []string{"FLY_API_TOKEN", "FLY_APP_NAME"}
	case "ibm-openwhisk":
		return []string{"IBM_CLOUD_API_KEY", "IBM_CLOUD_REGION", "IBM_CLOUD_NAMESPACE"}
	default:
		return nil
	}
}

// MissingProviderEnvVars returns env var names that are required for the provider but missing or empty in the environment.
func MissingProviderEnvVars(provider string) []string {
	required := RequiredProviderEnvVars(provider)
	if len(required) == 0 {
		return nil
	}
	var missing []string
	for _, name := range required {
		if v := os.Getenv(name); strings.TrimSpace(v) == "" {
			missing = append(missing, name)
		}
	}
	return missing
}
