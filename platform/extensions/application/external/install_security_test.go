package external

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestResolvePluginExecutable_RejectsAbsoluteAndEscape(t *testing.T) {
	dir := t.TempDir()

	if _, err := resolvePluginExecutable(dir, "/bin/sh"); err == nil {
		t.Error("absolute executable path must be rejected")
	}
	if _, err := resolvePluginExecutable(dir, "../../bin/evil"); err == nil {
		t.Error("escaping executable path must be rejected")
	}
	if _, err := resolvePluginExecutable(dir, ""); err == nil {
		t.Error("empty executable must be rejected")
	}

	got, err := resolvePluginExecutable(dir, "bin/plugin")
	if err != nil {
		t.Fatalf("valid relative executable rejected: %v", err)
	}
	if want := filepath.Join(dir, "bin/plugin"); got != want {
		t.Errorf("resolved = %q, want %q", got, want)
	}
}

func TestVerifyChecksumsIfPresent(t *testing.T) {
	dir := t.TempDir()
	payload := []byte("plugin-binary")
	if err := os.WriteFile(filepath.Join(dir, "plugin"), payload, 0o755); err != nil {
		t.Fatal(err)
	}

	// No checksums.txt -> allowed.
	if err := verifyChecksumsIfPresent(dir); err != nil {
		t.Fatalf("missing checksums.txt should be allowed, got %v", err)
	}

	// Correct checksum -> allowed.
	sum := sha256.Sum256(payload)
	good := hex.EncodeToString(sum[:]) + "  plugin\n"
	if err := os.WriteFile(filepath.Join(dir, "checksums.txt"), []byte(good), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := verifyChecksumsIfPresent(dir); err != nil {
		t.Fatalf("matching checksum should pass, got %v", err)
	}

	// Tampered checksum -> must error (regression: result was discarded).
	bad := "deadbeef  plugin\n"
	if err := os.WriteFile(filepath.Join(dir, "checksums.txt"), []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := verifyChecksumsIfPresent(dir); err == nil {
		t.Fatal("checksum mismatch must be reported, not ignored")
	}
}

func TestValidateSecureURL(t *testing.T) {
	cases := []struct {
		url     string
		wantErr bool
	}{
		{"https://registry.runfabric.cloud", false},
		{"https://evil.example/x.tgz", false}, // https allowed (host trust is separate)
		{"http://localhost:8080/x", false},    // loopback http allowed for dev
		{"http://127.0.0.1/x", false},
		{"http://attacker.host/x", true},   // plaintext non-loopback refused
		{"http://169.254.169.254/x", true}, // metadata endpoint over http refused
		{"ftp://host/x", true},             // unsupported scheme
		{"https:///x", true},               // no host
	}
	for _, c := range cases {
		err := validateSecureURL(c.url)
		if (err != nil) != c.wantErr {
			t.Errorf("validateSecureURL(%q) err=%v, wantErr=%v", c.url, err, c.wantErr)
		}
	}
}
