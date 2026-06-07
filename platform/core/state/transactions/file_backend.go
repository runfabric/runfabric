package transactions

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/runfabric/runfabric/platform/core/model/errors"
	statecore "github.com/runfabric/runfabric/platform/core/state/core"
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
	path := b.path(j.Service, j.Stage)

	if existing, err := b.Load(j.Service, j.Stage); err == nil {
		// Every Journal write advances the version, so a write must be strictly
		// greater than what is on disk. An equal (or lower) incoming version means
		// a concurrent writer already persisted this revision — reject it instead
		// of silently overwriting their changes (lost update).
		if j.Version <= existing.Version {
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

	// UpdatedAt and Checksum are stamped by the Journal layer before Save so the
	// persisted checksum matches the bytes written. The backend must not mutate
	// the file here, or the on-disk checksum would never verify.
	data, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal journal: %w", err)
	}

	if err := statecore.WriteStateFile(path, data); err != nil {
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
