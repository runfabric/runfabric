package codec

import (
	"encoding/json"

	providers "github.com/runfabric/runfabric/internal/provider/contracts"
	"github.com/runfabric/runfabric/platform/core/model/config"
)

// FromCoreConfig converts a core Config to a transport-safe provider Config.
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
