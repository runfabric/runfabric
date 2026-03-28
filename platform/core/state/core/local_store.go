package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const CurrentReceiptVersion = 2

type FunctionDeployment struct {
	Function           string            `json:"function"`
	ArtifactSHA256     string            `json:"artifactSha256"`
	ConfigSignature    string            `json:"configSignature"`
	ResourceName       string            `json:"resourceName,omitempty"`
	ResourceIdentifier string            `json:"resourceIdentifier,omitempty"`
	Metadata           map[string]string `json:"metadata,omitempty"`
	EnvironmentHash    string            `json:"environmentHash,omitempty"`
	TagsHash           string            `json:"tagsHash,omitempty"`
	LayersHash         string            `json:"layersHash,omitempty"`
}

type Artifact struct {
	Function        string `json:"function"`
	Runtime         string `json:"runtime"`
	SourcePath      string `json:"sourcePath"`
	OutputPath      string `json:"outputPath"`
	SHA256          string `json:"sha256"`
	SizeBytes       int64  `json:"sizeBytes"`
	ConfigSignature string `json:"configSignature,omitempty"`
}

// Receipt is the deployment receipt stored per stage. Metadata is the versioned place for
// structured metadata (e.g. compiled graph hash/version, run summaries) without breaking existing providers.
type Receipt struct {
	Version      int                  `json:"version"`
	Service      string               `json:"service"`
	Stage        string               `json:"stage"`
	Provider     string               `json:"provider"`
	DeploymentID string               `json:"deploymentId"`
	Outputs      map[string]string    `json:"outputs"`
	Artifacts    []Artifact           `json:"artifacts,omitempty"`
	Metadata     map[string]string    `json:"metadata,omitempty"` // Phase 13.11.6: use for graph hash, run summaries, etc.
	Functions    []FunctionDeployment `json:"functions,omitempty"`
	UpdatedAt    string               `json:"updatedAt"`
}

func Save(root string, receipt *Receipt) error {
	if receipt == nil {
		return fmt.Errorf("nil receipt")
	}
	dir := filepath.Join(root, ".runfabric")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}

	receipt.Version = CurrentReceiptVersion
	receipt.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	path := filepath.Join(dir, receipt.Stage+".json")
	data, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal receipt: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write receipt: %w", err)
	}
	return nil
}

func Load(root, stage string) (*Receipt, error) {
	path := filepath.Join(root, ".runfabric", stage+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read receipt: %w", err)
	}

	var r Receipt
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("unmarshal receipt: %w", err)
	}

	migrated, err := MigrateReceipt(&r)
	if err != nil {
		return nil, err
	}
	return migrated, nil
}

func Delete(root, stage string) error {
	path := filepath.Join(root, ".runfabric", stage+".json")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete receipt: %w", err)
	}
	return nil
}

// ReleaseEntry is one deployment (stage + timestamp) for deploy list / releases.
type ReleaseEntry struct {
	Stage     string `json:"stage"`
	UpdatedAt string `json:"updatedAt"`
}

// ListReleases returns all receipt stages and their UpdatedAt from the local .runfabric dir.
func ListReleases(root string) ([]ReleaseEntry, error) {
	dir := filepath.Join(root, ".runfabric")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("list receipts: %w", err)
	}
	var out []ReleaseEntry
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if len(name) > 5 && name[len(name)-5:] == ".json" {
			stage := name[:len(name)-5]
			r, err := Load(root, stage)
			if err != nil {
				continue
			}
			out = append(out, ReleaseEntry{Stage: stage, UpdatedAt: r.UpdatedAt})
		}
	}
	return out, nil
}
