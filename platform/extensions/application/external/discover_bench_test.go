package external

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"gopkg.in/yaml.v3"
)

func BenchmarkDiscover(b *testing.B) {
	home := b.TempDir()
	b.Setenv(envHome, home)

	for i := 0; i < 20; i++ {
		id := "bench-provider-" + strconv.Itoa(i)
		version := "1.0." + strconv.Itoa(i)
		dir := filepath.Join(home, "plugins", "providers", id, version)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			b.Fatalf("mkdir: %v", err)
		}
		execPath := filepath.Join(dir, "plugin")
		if err := os.WriteFile(execPath, []byte("#!/bin/sh\necho ok\n"), 0o755); err != nil {
			b.Fatalf("write executable: %v", err)
		}
		manifest := pluginYAML{
			APIVersion: "runfabric.dev/v1",
			Kind:       "provider",
			ID:         id,
			Name:       id,
			Version:    version,
			Executable: "./plugin",
		}
		raw, err := yaml.Marshal(manifest)
		if err != nil {
			b.Fatalf("marshal manifest: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "plugin.yaml"), raw, 0o644); err != nil {
			b.Fatalf("write plugin.yaml: %v", err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Discover(DiscoverOptions{IncludeInvalid: true}); err != nil {
			b.Fatalf("discover: %v", err)
		}
	}
}
