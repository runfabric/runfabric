package locking

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type FileLock struct {
	Path string
	Held bool
}

func Acquire(root, service, stage string) (*FileLock, error) {
	lockDir := filepath.Join(root, ".runfabric", "locks")
	if err := os.MkdirAll(lockDir, 0o755); err != nil {
		return nil, fmt.Errorf("create lock dir: %w", err)
	}

	path := filepath.Join(lockDir, fmt.Sprintf("%s-%s.lock", service, stage))

	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return nil, fmt.Errorf("lock already held for service=%s stage=%s", service, stage)
		}
		return nil, fmt.Errorf("acquire lock: %w", err)
	}
	defer f.Close()

	_, _ = f.WriteString(time.Now().UTC().Format(time.RFC3339))

	return &FileLock{
		Path: path,
		Held: true,
	}, nil
}

func (l *FileLock) Release() error {
	if l == nil || !l.Held {
		return nil
	}
	if err := os.Remove(l.Path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("release lock: %w", err)
	}
	l.Held = false
	return nil
}
