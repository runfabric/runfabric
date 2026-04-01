package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	runFabricStateFilePrefix = "runfabric-state-"
)

// RunFabricEndpoint is one deployed endpoint in the runtime fabric (e.g. one provider/region).
type RunFabricEndpoint struct {
	Provider  string `json:"provider"`
	URL       string `json:"url"`
	UpdatedAt string `json:"updatedAt,omitempty"`
	Healthy   *bool  `json:"healthy,omitempty"` // set by fabric health check
}

// RunFabricState holds the list of endpoints for a stage (active-active).
type RunFabricState struct {
	Service   string              `json:"service"`
	Stage     string              `json:"stage"`
	Endpoints []RunFabricEndpoint `json:"endpoints"`
	UpdatedAt string              `json:"updatedAt"`
}

// LoadRunFabricState reads state from .runfabric/runfabric-state-<stage>.json.
func LoadRunFabricState(root, stage string) (*RunFabricState, error) {
	path := runFabricStatePath(root, stage)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read runfabric state: %w", err)
	}
	var s RunFabricState
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("unmarshal runfabric state: %w", err)
	}
	return &s, nil
}

// SaveRunFabricState writes state to .runfabric/runfabric-state-<stage>.json.
func SaveRunFabricState(root string, s *RunFabricState) error {
	dir := filepath.Join(root, ".runfabric")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}
	s.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	path := runFabricStatePath(root, s.Stage)
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal runfabric state: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write runfabric state: %w", err)
	}
	return nil
}

func runFabricStatePath(root, stage string) string {
	return filepath.Join(root, ".runfabric", runFabricStateFilePrefix+stage+".json")
}
