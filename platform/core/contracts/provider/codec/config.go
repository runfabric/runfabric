package codec

import (
	"encoding/json"

	providers "github.com/runfabric/runfabric/platform/core/contracts/provider"
	"github.com/runfabric/runfabric/platform/core/model/config"
)

// FromCoreConfig converts a core Config to a transport-safe provider Config.
//
// The top-level secret store (cfg.Secrets) is dropped before serialization: it
// is the source used during resolution, but providers deploy from each
// function's already-resolved Environment/Secrets, so shipping the central
// secret map to every external plugin over stdio would over-expose secrets a
// plugin never needs. Per-function resolved values are retained because the
// provider needs them to inject secrets into the deployed function.
func FromCoreConfig(cfg *config.Config) (providers.Config, error) {
	if cfg == nil {
		return nil, nil
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	var sdkCfg providers.Config
	if err := json.Unmarshal(b, &sdkCfg); err != nil {
		return nil, err
	}
	// Drop the central secret store from the wire payload entirely.
	delete(sdkCfg, "Secrets")
	return sdkCfg, nil
}

// ToCoreConfig converts a transport-safe provider Config to a core Config.
func ToCoreConfig(cfg providers.Config) (*config.Config, error) {
	if cfg == nil {
		return nil, nil
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	var coreCfg config.Config
	if err := json.Unmarshal(b, &coreCfg); err != nil {
		return nil, err
	}
	return &coreCfg, nil
}
