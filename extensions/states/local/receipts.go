package local

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	statetypes "github.com/runfabric/runfabric/extensions/types"
)

const currentReceiptVersion = 2

type ReceiptBackend struct {
	Root string
}

func NewReceiptBackend(root string) *ReceiptBackend {
	return &ReceiptBackend{Root: root}
}

func (b *ReceiptBackend) Load(stage string) (*statetypes.Receipt, error) {
	path := filepath.Join(b.Root, ".runfabric", stage+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read receipt: %w", err)
	}
	var r statetypes.Receipt
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("unmarshal receipt: %w", err)
	}
	if r.Version != currentReceiptVersion {
		return nil, fmt.Errorf("unsupported receipt version %d", r.Version)
	}
	return &r, nil
}

func (b *ReceiptBackend) Save(receipt *statetypes.Receipt) error {
	if receipt == nil {
		return fmt.Errorf("nil receipt")
	}

	dir := filepath.Join(b.Root, ".runfabric")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}

	receipt.Version = currentReceiptVersion
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

func (b *ReceiptBackend) Delete(stage string) error {
	path := filepath.Join(b.Root, ".runfabric", stage+".json")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete receipt: %w", err)
	}
	return nil
}

func (b *ReceiptBackend) ListReleases() ([]statetypes.ReleaseEntry, error) {
	dir := filepath.Join(b.Root, ".runfabric")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("list receipts: %w", err)
	}
	var out []statetypes.ReleaseEntry
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		stage := strings.TrimSuffix(name, ".json")
		r, err := b.Load(stage)
		if err != nil {
			continue
		}
		out = append(out, statetypes.ReleaseEntry{Stage: stage, UpdatedAt: r.UpdatedAt})
	}
	return out, nil
}

func (b *ReceiptBackend) Kind() string {
	return "local"
}
