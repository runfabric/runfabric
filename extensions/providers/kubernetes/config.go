package kubernetes

import (
	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// ProviderConfig is the typed representation of the provider: block in runfabric.yml
// for the kubernetes provider.
type ProviderConfig struct {
	Runtime     string `json:"runtime,omitempty"`
	ServiceType string `json:"serviceType,omitempty"` // ClusterIP | NodePort | LoadBalancer
	Image       string `json:"image,omitempty"`
	Namespace   string `json:"namespace,omitempty"`
}

// DeployConfig is the typed representation of the deploy: block.
type DeployConfig struct {
	Strategy string `json:"strategy,omitempty"` // all-at-once | per-function
}

// ExtensionsConfig holds router sub-config relevant to this provider.
type ExtensionsConfig struct {
	Router RouterConfig `json:"router,omitempty"`
}

// RouterConfig is the typed representation of extensions.router in runfabric.yml.
type RouterConfig struct {
	Hostname string `json:"hostname,omitempty"`
}

// kubeConfig is the fully decoded, typed configuration for one deploy call.
type kubeConfig struct {
	Service    string
	Stage      string
	Provider   ProviderConfig
	Deploy     DeployConfig
	Extensions ExtensionsConfig
}

// decodeKubeConfig decodes the SDK config map into a typed kubeConfig.
func decodeKubeConfig(cfg sdkprovider.Config, stage string) (kubeConfig, error) {
	kc := kubeConfig{
		Service: sdkprovider.Service(cfg),
		Stage:   stage,
	}

	if err := sdkprovider.DecodeConfig(cfg, &kc.Provider); err != nil {
		return kc, err
	}
	if err := sdkprovider.DecodeSection(cfg, "deploy", &kc.Deploy); err != nil {
		return kc, err
	}
	if err := sdkprovider.DecodeSection(cfg, "extensions", &kc.Extensions); err != nil {
		return kc, err
	}

	// Env var fallbacks for fields not set in YAML.
	if kc.Provider.Image == "" {
		kc.Provider.Image = sdkprovider.Env("KUBERNETES_IMAGE")
	}
	if kc.Provider.ServiceType == "" {
		kc.Provider.ServiceType = sdkprovider.Env("KUBERNETES_SERVICE_TYPE")
	}

	return kc, nil
}
