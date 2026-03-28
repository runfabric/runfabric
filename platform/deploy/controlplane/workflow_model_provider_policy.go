package controlplane

import "strings"

type providerModelPolicy struct {
	selector ModelSelector
	shaper   ModelOutputShaper
}

var providerModelPolicies = map[string]providerModelPolicy{
	"aws-lambda": {
		selector: AWSModelSelector{},
		shaper:   AWSBedrockOutputShaper{},
	},
	"gcp-functions": {
		selector: GCPModelSelector{},
		shaper:   GCPVertexOutputShaper{},
	},
	"azure-functions": {
		selector: AzureModelSelector{},
		shaper:   AzureOpenAIOutputShaper{},
	},
}

func providerModelPolicyFor(provider string) (providerModelPolicy, bool) {
	policy, ok := providerModelPolicies[normalizedProviderID(provider)]
	return policy, ok
}

func normalizedProviderID(provider string) string {
	return strings.ToLower(strings.TrimSpace(provider))
}
