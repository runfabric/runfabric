package sdkbridge

import (
	"encoding/json"

	"github.com/runfabric/runfabric/platform/core/model/config"
	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// FromCoreConfig converts a typed core config to the SDK's schema-free config map.
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

// ToCoreConfig converts the SDK's schema-free config map to a typed core config.
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
