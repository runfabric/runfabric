package sdkbridge

import (
	"encoding/json"

	"github.com/runfabric/runfabric/platform/core/model/config"
	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// FromCoreConfig converts a core Config to an SDK provider Config.
func FromCoreConfig(cfg *config.Config) (sdkprovider.Config, error) {
	if cfg == nil {
		return nil, nil
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	var sdkCfg sdkprovider.Config
	if err := json.Unmarshal(b, &sdkCfg); err != nil {
		return nil, err
	}
	return sdkCfg, nil
}

// ToCoreConfig converts an SDK provider Config to a core Config.
func ToCoreConfig(cfg sdkprovider.Config) (*config.Config, error) {
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
