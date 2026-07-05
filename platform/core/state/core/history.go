package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// maxHistoryEntries caps retained snapshots per stage. Deploys are infrequent,
// but the directory is bounded so it cannot grow without limit.
const maxHistoryEntries = 20

// HistoryEntry identifies one retained past receipt for a stage. It is the
// engine-side record of a prior release: enough to inspect what was deployed and
// when, and to load the full receipt on demand.
type HistoryEntry struct {
	ID           string `json:"id"`
	Stage        string `json:"stage"`
	DeploymentID string `json:"deploymentId"`
	Provider     string `json:"provider,omitempty"`
	UpdatedAt    string `json:"updatedAt"`
}

func historyDir(root, stage string) string {
	return filepath.Join(root, ".runfabric", "history", stage)
}

// historyID builds a filesystem-safe, lexically-sortable id from a receipt's
// timestamp and deployment id. Because it is prefixed with the RFC3339
// timestamp, lexical sort order equals chronological order.
func historyID(updatedAt, deploymentID string) string {
	stamp := strings.ReplaceAll(updatedAt, ":", "-")
	if deploymentID == "" {
		return stamp
	}
	return stamp + "_" + sanitizeSegment(deploymentID)
}

func sanitizeSegment(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			return r
		default:
			return '-'
		}
	}, s)
}

// snapshotHistory copies the current head receipt for a stage (if one exists)
// into the stage's history directory before it is overwritten. Best-effort: any
// failure is swallowed so it never blocks the deploy that triggered it.
func snapshotHistory(root, stage string) {
	if stage == "" {
		return
	}
	headPath := filepath.Join(root, ".runfabric", stage+".json")
	data, err := os.ReadFile(headPath)
	if err != nil {
		return // no prior head → nothing to snapshot
	}
	var r Receipt
	if err := json.Unmarshal(data, &r); err != nil || r.UpdatedAt == "" {
		return
	}
	dst := filepath.Join(historyDir(root, stage), historyID(r.UpdatedAt, r.DeploymentID)+".json")
	if _, err := os.Stat(dst); err == nil {
		return // already snapshotted (idempotent)
	}
	if err := WriteStateFile(dst, data); err != nil {
		return
	}
	pruneHistory(root, stage)
}

// pruneHistory keeps only the most recent maxHistoryEntries snapshots.
func pruneHistory(root, stage string) {
	dir := historyDir(root, stage)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			names = append(names, e.Name())
		}
	}
	if len(names) <= maxHistoryEntries {
		return
	}
	sort.Strings(names) // lexical == chronological (ids start with the timestamp)
	for _, name := range names[:len(names)-maxHistoryEntries] {
		_ = os.Remove(filepath.Join(dir, name))
	}
}

// ListStageHistory returns the retained past receipts for a stage, newest first.
// A stage with no history (never redeployed) yields an empty list, not an error.
func ListStageHistory(root, stage string) ([]HistoryEntry, error) {
	dir := historyDir(root, stage)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("list history: %w", err)
	}
	var out []HistoryEntry
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var r Receipt
		if err := json.Unmarshal(data, &r); err != nil {
			continue
		}
		out = append(out, HistoryEntry{
			ID:           strings.TrimSuffix(e.Name(), ".json"),
			Stage:        stage,
			DeploymentID: r.DeploymentID,
			Provider:     r.Provider,
			UpdatedAt:    r.UpdatedAt,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID }) // newest first
	return out, nil
}

// LoadHistoryEntry loads a specific retained receipt for a stage by id.
func LoadHistoryEntry(root, stage, id string) (*Receipt, error) {
	data, err := os.ReadFile(filepath.Join(historyDir(root, stage), id+".json"))
	if err != nil {
		return nil, fmt.Errorf("read history entry %q: %w", id, err)
	}
	var r Receipt
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("unmarshal history entry %q: %w", id, err)
	}
	return MigrateReceipt(&r)
}
