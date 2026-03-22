package transactions

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/runfabric/runfabric/platform/core/model/errors"
)

type FileBackend struct {
	Root string
}

func NewFileBackend(root string) *FileBackend {
	return &FileBackend{Root: root}
}

func (b *FileBackend) path(service, stage string) string {
	return filepath.Join(b.Root, ".runfabric", "journals", service+"-"+stage+".journal.json")
}

func (b *FileBackend) Load(service, stage string) (*JournalFile, error) {
	path := b.path(service, stage)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var j JournalFile
	if err := json.Unmarshal(data, &j); err != nil {
		return nil, fmt.Errorf("parse journal: %w", err)
	}
	return &j, nil
}

func (b *FileBackend) Save(j *JournalFile) error {
	dir := filepath.Join(b.Root, ".runfabric", "journals")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create journal dir: %w", err)
	}

	path := b.path(j.Service, j.Stage)

	if existing, err := b.Load(j.Service, j.Stage); err == nil {
		if j.Version < existing.Version {
			return &errors.ConflictError{
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

	j.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	data, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal journal: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write journal: %w", err)
	}
	return nil
}

func (b *FileBackend) Delete(service, stage string) error {
	path := b.path(service, stage)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete journal: %w", err)
	}
	return nil
}
