package local

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	statetypes "github.com/runfabric/runfabric/extensions/types"
)

type JournalBackend struct {
	Root string
}

func NewJournalBackend(root string) *JournalBackend {
	return &JournalBackend{Root: root}
}

func (b *JournalBackend) Kind() string {
	return "local"
}

func (b *JournalBackend) path(service, stage string) string {
	return filepath.Join(b.Root, ".runfabric", "journals", service+"-"+stage+".journal.json")
}

func (b *JournalBackend) Load(service, stage string) (*statetypes.JournalFile, error) {
	path := b.path(service, stage)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var j statetypes.JournalFile
	if err := json.Unmarshal(data, &j); err != nil {
		return nil, fmt.Errorf("parse journal: %w", err)
	}
	return &j, nil
}

func (b *JournalBackend) Save(j *statetypes.JournalFile) error {
	if j == nil {
		return fmt.Errorf("nil journal")
	}

	dir := filepath.Join(b.Root, ".runfabric", "journals")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create journal dir: %w", err)
	}

	path := b.path(j.Service, j.Stage)
	if existing, err := b.Load(j.Service, j.Stage); err == nil && existing != nil {
		if j.Version < existing.Version {
			return &statetypes.ConflictError{
				Backend:         "file",
				Service:         j.Service,
				Stage:           j.Stage,
				Resource:        "journal",
				CurrentVersion:  existing.Version,
				IncomingVersion: j.Version,
				Action:          "inspect journal and retry with latest state",
			}
		}
	}

	data, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal journal: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write journal: %w", err)
	}
	return nil
}

func (b *JournalBackend) Delete(service, stage string) error {
	path := b.path(service, stage)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete journal: %w", err)
	}
	return nil
}
