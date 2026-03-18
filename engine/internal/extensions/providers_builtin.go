package extensions

import (
	awsprovider "github.com/runfabric/runfabric/engine/internal/extensions/provider/aws"
	gcpprovider "github.com/runfabric/runfabric/engine/internal/extensions/provider/gcp"
	"github.com/runfabric/runfabric/engine/internal/extensions/providers"
)

// NewBuiltinProviderRegistry returns a providers.Registry populated with all built-in
// provider plugins (AWS, GCP). Built-in providers are part of the extensions model;
// bootstrap and lifecycle use this registry to resolve provider by config name.
func NewBuiltinProviderRegistry() *providers.Registry {
	reg := providers.NewRegistry()
	aws := awsprovider.New()
	reg.Register(aws)
	reg.Register(providers.NewNamedProvider("aws-lambda", aws))
	gcp := gcpprovider.New()
	reg.Register(gcp)
	return reg
}
