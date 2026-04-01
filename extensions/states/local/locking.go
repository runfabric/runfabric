package local

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	statetypes "github.com/runfabric/runfabric/extensions/types"
)

type LockBackend struct {
	Root string
}

func NewLockBackend(root string) *LockBackend {
	return &LockBackend{Root: root}
}

func (b *LockBackend) Kind() string {
	return "local"
}

func (b *LockBackend) lockPath(service, stage string) string {
	return filepath.Join(b.Root, ".runfabric", "locks", service+"-"+stage+".lock.json")
}

func (b *LockBackend) Acquire(service, stage, operation string, staleAfter time.Duration) (*statetypes.Handle, error) {
	lockDir := filepath.Join(b.Root, ".runfabric", "locks")
	if err := os.MkdirAll(lockDir, 0o755); err != nil {
		return nil, fmt.Errorf("create lock dir: %w", err)
	}

	path := b.lockPath(service, stage)
	if existing, err := b.Read(service, stage); err == nil && existing != nil {
		expiresAt, parseErr := time.Parse(time.RFC3339, existing.ExpiresAt)
		if parseErr == nil && time.Now().UTC().After(expiresAt) {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return nil, fmt.Errorf("remove expired lock: %w", err)
			}
		} else {
			return nil, fmt.Errorf(
				"lock already held for service=%s stage=%s operation=%s owner=%s",
				existing.Service,
				existing.Stage,
				existing.Operation,
				existing.OwnerToken,
			)
		}
	}

	token, err := randomToken()
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	record := statetypes.LockRecord{
		Service:         service,
		Stage:           stage,
		Operation:       operation,
		OwnerToken:      token,
		PID:             os.Getpid(),
		CreatedAt:       now.Format(time.RFC3339),
		ExpiresAt:       now.Add(staleAfter).Format(time.RFC3339),
		LastHeartbeatAt: now.Format(time.RFC3339),
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return nil, fmt.Errorf("lock already exists for service=%s stage=%s", service, stage)
		}
		return nil, fmt.Errorf("acquire lock: %w", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(&record); err != nil {
		return nil, fmt.Errorf("write lock file: %w", err)
	}

	return statetypes.NewHandle(service, stage, token, b, b, nil), nil
}

func (b *LockBackend) Read(service, stage string) (*statetypes.LockRecord, error) {
	path := b.lockPath(service, stage)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var record statetypes.LockRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, fmt.Errorf("parse lock file: %w", err)
	}
	return &record, nil
}

func (b *LockBackend) Release(service, stage string) error {
	path := b.lockPath(service, stage)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("release lock: %w", err)
	}
	return nil
}

func (b *LockBackend) Renew(service, stage, ownerToken string, leaseFor time.Duration) error {
	record, err := b.Read(service, stage)
	if err != nil {
		return err
	}
	if record.OwnerToken != ownerToken {
		return fmt.Errorf("lock owner mismatch")
	}

	now := time.Now().UTC()
	record.ExpiresAt = now.Add(leaseFor).Format(time.RFC3339)
	record.LastHeartbeatAt = now.Format(time.RFC3339)

	path := b.lockPath(service, stage)
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func randomToken() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
