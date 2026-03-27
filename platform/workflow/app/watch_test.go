package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatchProjectDir_IgnoresBuildOutputDirs(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "runfabric.yml")
	if err := os.WriteFile(configPath, []byte("service: test\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	srcDir := filepath.Join(root, "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}
	distDir := filepath.Join(root, "dist")
	if err := os.MkdirAll(distDir, 0o755); err != nil {
		t.Fatalf("mkdir dist: %v", err)
	}
	srcFile := filepath.Join(srcDir, "handler.ts")
	if err := os.WriteFile(srcFile, []byte("export const handler = 1\n"), 0o644); err != nil {
		t.Fatalf("write src: %v", err)
	}
	distFile := filepath.Join(distDir, "handler.js")
	if err := os.WriteFile(distFile, []byte("exports.handler = 1\n"), 0o644); err != nil {
		t.Fatalf("write dist: %v", err)
	}

	done := make(chan struct{})
	defer close(done)
	changes := WatchProjectDir(configPath, 30*time.Millisecond, done)

	// Let watcher capture initial snapshot.
	time.Sleep(80 * time.Millisecond)

	if err := os.WriteFile(distFile, []byte("exports.handler = 2\n"), 0o644); err != nil {
		t.Fatalf("rewrite dist: %v", err)
	}
	select {
	case <-changes:
		t.Fatal("dist change should be ignored")
	case <-time.After(160 * time.Millisecond):
	}

	if err := os.WriteFile(srcFile, []byte("export const handler = 2\n"), 0o644); err != nil {
		t.Fatalf("rewrite src: %v", err)
	}
	select {
	case <-changes:
		// expected
	case <-time.After(300 * time.Millisecond):
		t.Fatal("expected src change to be detected")
	}
}
