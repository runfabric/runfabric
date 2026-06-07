package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// State files may contain sensitive data (deploy outputs, workflow I/O,
// connection strings). Keep them owner-only and write them atomically so a
// crash mid-write cannot corrupt the file every later command reads.
const (
	// StateDirPerm is the permission for state directories under .runfabric.
	StateDirPerm os.FileMode = 0o700
	// StateFilePerm is the permission for state files under .runfabric.
	StateFilePerm os.FileMode = 0o600
)

// EnsureStateDir creates dir (and parents) with owner-only permissions. If dir
// already exists with looser permissions (e.g. created before state hardening),
// it is tightened to StateDirPerm.
func EnsureStateDir(dir string) error {
	if err := os.MkdirAll(dir, StateDirPerm); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}
	// MkdirAll does not change the mode of an already-existing directory, so
	// tighten the leaf explicitly. Platforms that don't support chmod report
	// ErrUnsupported, which is not an error we care about here.
	if err := os.Chmod(dir, StateDirPerm); err != nil && !errors.Is(err, errors.ErrUnsupported) {
		return fmt.Errorf("chmod state dir: %w", err)
	}
	return nil
}

// WriteStateFile atomically writes data to path with owner-only permissions.
// It writes to a temp file in the same directory, flushes it to disk, and
// renames into place so readers never observe a partially written file and a
// crash cannot leave a corrupt destination.
func WriteStateFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := EnsureStateDir(dir); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".tmp-"+filepath.Base(path)+"-*")
	if err != nil {
		return fmt.Errorf("create temp state file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) // no-op after successful rename

	// os.CreateTemp already creates the file 0o600, but set it explicitly so the
	// guarantee does not depend on that default. Ignore ErrUnsupported (Windows).
	if err := tmp.Chmod(StateFilePerm); err != nil && !errors.Is(err, errors.ErrUnsupported) {
		tmp.Close()
		return fmt.Errorf("chmod temp state file: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp state file: %w", err)
	}
	// Flush file contents to disk before the rename so a crash cannot leave the
	// renamed destination pointing at unwritten data.
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync temp state file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp state file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("move state file into place: %w", err)
	}
	// Flush the directory entry so the rename itself survives a crash. A failure
	// here (e.g. platforms that disallow opening a directory) is best-effort.
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	return nil
}
