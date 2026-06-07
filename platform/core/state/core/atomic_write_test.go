package core

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestWriteStateFilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix file modes not enforced on windows")
	}
	dir := filepath.Join(t.TempDir(), ".runfabric", "runs", "dev")
	path := filepath.Join(dir, "run.json")

	if err := WriteStateFile(path, []byte("payload")); err != nil {
		t.Fatalf("WriteStateFile: %v", err)
	}

	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	if got := fi.Mode().Perm(); got != StateFilePerm {
		t.Errorf("file perm = %o, want %o", got, StateFilePerm)
	}

	di, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if got := di.Mode().Perm(); got != StateDirPerm {
		t.Errorf("dir perm = %o, want %o", got, StateDirPerm)
	}
}

func TestWriteStateFileTightensExistingDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix file modes not enforced on windows")
	}
	root := t.TempDir()
	dir := filepath.Join(root, ".runfabric")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("pre-create dir: %v", err)
	}

	if err := WriteStateFile(filepath.Join(dir, "state.json"), []byte("x")); err != nil {
		t.Fatalf("WriteStateFile: %v", err)
	}

	di, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if got := di.Mode().Perm(); got != StateDirPerm {
		t.Errorf("existing dir perm = %o, want %o (should be tightened)", got, StateDirPerm)
	}
}

func TestWriteStateFileOverwriteAndNoTempLeak(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".runfabric")
	path := filepath.Join(dir, "state.json")

	if err := WriteStateFile(path, []byte("first")); err != nil {
		t.Fatalf("first write: %v", err)
	}
	if err := WriteStateFile(path, []byte("second")); err != nil {
		t.Fatalf("second write: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(got) != "second" {
		t.Errorf("content = %q, want %q", got, "second")
	}

	// No leftover temp files in the directory.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	for _, e := range entries {
		if e.Name() != filepath.Base(path) {
			t.Errorf("unexpected leftover file in state dir: %s", e.Name())
		}
	}
}
