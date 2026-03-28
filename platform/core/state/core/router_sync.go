package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type RouterSyncEndpoint struct {
	Name    string `json:"name"`
	URL     string `json:"url"`
	Weight  int    `json:"weight,omitempty"`
	Healthy *bool  `json:"healthy,omitempty"`
}

type RouterSyncRouting struct {
	Contract   string               `json:"contract"`
	Service    string               `json:"service"`
	Stage      string               `json:"stage"`
	Hostname   string               `json:"hostname"`
	Strategy   string               `json:"strategy"`
	HealthPath string               `json:"healthPath,omitempty"`
	TTL        int                  `json:"ttl,omitempty"`
	Endpoints  []RouterSyncEndpoint `json:"endpoints"`
}

type RouterSyncAction struct {
	Resource string `json:"resource"`
	Action   string `json:"action"`
	Name     string `json:"name"`
	Detail   string `json:"detail,omitempty"`
}

type RouterSyncActionSummary struct {
	Create          int `json:"create"`
	Update          int `json:"update"`
	Noop            int `json:"noop"`
	DeleteCandidate int `json:"deleteCandidate,omitempty"`
}

type RouterSyncEvent struct {
	Timestamp string                  `json:"timestamp"`
	Phase     string                  `json:"phase"`
	Message   string                  `json:"message"`
	Summary   RouterSyncActionSummary `json:"summary,omitempty"`
}

type RouterSyncSnapshot struct {
	ID        string                  `json:"id"`
	Service   string                  `json:"service"`
	Stage     string                  `json:"stage"`
	PluginID  string                  `json:"pluginId"`
	Operation string                  `json:"operation,omitempty"`
	Trigger   string                  `json:"trigger,omitempty"`
	ZoneID    string                  `json:"zoneId,omitempty"`
	AccountID string                  `json:"accountId,omitempty"`
	DryRun    bool                    `json:"dryRun"`
	CreatedAt string                  `json:"createdAt"`
	Routing   RouterSyncRouting       `json:"routing"`
	Before    []RouterSyncAction      `json:"before,omitempty"`
	Actions   []RouterSyncAction      `json:"actions,omitempty"`
	After     []RouterSyncAction      `json:"after,omitempty"`
	Events    []RouterSyncEvent       `json:"events,omitempty"`
	Summary   RouterSyncActionSummary `json:"summary,omitempty"`
	BeforeSum RouterSyncActionSummary `json:"beforeSummary,omitempty"`
	AfterSum  RouterSyncActionSummary `json:"afterSummary,omitempty"`
}

func LoadRouterSyncHistory(root, stage string) ([]RouterSyncSnapshot, error) {
	path := filepath.Join(root, ".runfabric", "router-sync-"+stage+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []RouterSyncSnapshot{}, nil
		}
		return nil, fmt.Errorf("read router sync history: %w", err)
	}
	var history []RouterSyncSnapshot
	if err := json.Unmarshal(data, &history); err != nil {
		return nil, fmt.Errorf("unmarshal router sync history: %w", err)
	}
	return history, nil
}

func SaveRouterSyncHistory(root, stage string, history []RouterSyncSnapshot) error {
	dir := filepath.Join(root, ".runfabric")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}
	path := filepath.Join(dir, "router-sync-"+stage+".json")
	data, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal router sync history: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write router sync history: %w", err)
	}
	return nil
}

func AppendRouterSyncSnapshot(root, stage string, snapshot RouterSyncSnapshot, keep int) error {
	history, err := LoadRouterSyncHistory(root, stage)
	if err != nil {
		return err
	}
	if snapshot.ID == "" {
		snapshot.ID = fmt.Sprintf("%d", time.Now().UTC().UnixNano())
	}
	if snapshot.CreatedAt == "" {
		snapshot.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	history = append(history, snapshot)
	if keep > 0 && len(history) > keep {
		history = history[len(history)-keep:]
	}
	return SaveRouterSyncHistory(root, stage, history)
}
