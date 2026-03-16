package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// FabricEndpoint is one deployed endpoint in the runtime fabric (e.g. one provider/region).
type FabricEndpoint struct {
	Provider  string `json:"provider"`
	URL       string `json:"url"`
	UpdatedAt string `json:"updatedAt,omitempty"`
	Healthy   *bool  `json:"healthy,omitempty"` // set by fabric health check
}

// FabricState holds the list of endpoints for a stage (active-active).
type FabricState struct {
	Service   string           `json:"service"`
	Stage     string           `json:"stage"`
	Endpoints []FabricEndpoint `json:"endpoints"`
	UpdatedAt string           `json:"updatedAt"`
}

// LoadFabricState reads fabric state from .runfabric/fabric-<stage>.json. Returns nil if file missing or invalid.
func LoadFabricState(root, stage string) (*FabricState, error) {
	path := filepath.Join(root, ".runfabric", "fabric-"+stage+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read fabric state: %w", err)
	}
	var s FabricState
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("unmarshal fabric state: %w", err)
	}
	return &s, nil
}

// SaveFabricState writes fabric state to .runfabric/fabric-<stage>.json.
func SaveFabricState(root string, s *FabricState) error {
	dir := filepath.Join(root, ".runfabric")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}
	s.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	path := filepath.Join(dir, "fabric-"+s.Stage+".json")
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal fabric state: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write fabric state: %w", err)
	}
	return nil
}
