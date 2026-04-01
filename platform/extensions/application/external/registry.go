package external

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	manifests "github.com/runfabric/runfabric/platform/extensions/manifest"
	"gopkg.in/yaml.v3"
)

const (
	envRegistryURL     = "RUNFABRIC_REGISTRY_URL"
	envRegistryToken   = "RUNFABRIC_REGISTRY_TOKEN"
	defaultRegistryURL = "https://registry.runfabric.cloud"

	// localDevPublicKeyID is used by the local registry scaffold to demonstrate
	// signature verification end-to-end.
	localDevPublicKeyID = "local-dev"
	// public key for ed25519.NewKeyFromSeed(seed=0x00..0x1f), base64-encoded.
	localDevPublicKeyB64 = "A6EHv/POEL4dcN0Y50vAmWfk1jCbpQ1fHdyGZBJVMbg="
)

func DefaultRegistryURL() string { return defaultRegistryURL }

func RegistryURLFromEnv() string {
	v := strings.TrimSpace(os.Getenv(envRegistryURL))
	if v == "" {
		return ""
	}
	return v
}

func RegistryTokenFromEnv() string {
	v := strings.TrimSpace(os.Getenv(envRegistryToken))
	if v == "" {
		return ""
	}
	return v
}

type ResolveResponse struct {
	Request struct {
		ID   string `json:"id"`
		Core string `json:"core"`
		OS   string `json:"os"`
		Arch string `json:"arch"`
	} `json:"request"`
	Resolved json.RawMessage `json:"resolved"`
	Meta     map[string]any  `json:"meta,omitempty"`
}

type ResolvedArtifact struct {
	Type      string `json:"type"`   // addon | binary
	Format    string `json:"format"` // tgz | zip | executable | ...
	URL       string `json:"url"`
	SizeBytes int64  `json:"sizeBytes"`
	Checksum  struct {
		Algorithm string `json:"algorithm"` // sha256
		Value     string `json:"value"`     // hex
	} `json:"checksum"`
	Signature *struct {
		Algorithm   string `json:"algorithm"` // ed25519
		Value       string `json:"value"`
		PublicKeyID string `json:"publicKeyId"`
	} `json:"signature,omitempty"`
}

type ResolvedPlugin struct {
	ID         string `json:"id"`
	Name       string `json:"name,omitempty"`
	Type       string `json:"type"` // plugin
	PluginKind string `json:"pluginKind"`
	Version    string `json:"version"`

	Description   string           `json:"description,omitempty"`
	Capabilities  []string         `json:"capabilities,omitempty"`
	Permissions   []string         `json:"permissions,omitempty"`
	Compatibility map[string]any   `json:"compatibility,omitempty"`
	Publisher     map[string]any   `json:"publisher,omitempty"`
	Artifact      ResolvedArtifact `json:"artifact"`
	Manifest      map[string]any   `json:"manifest,omitempty"`
	Integrity     map[string]any   `json:"integrity,omitempty"`
	Install       map[string]any   `json:"install,omitempty"`
}

type ResolveOptions struct {
	RegistryURL string // optional; default if empty
	AuthToken   string // optional; bearer token
	ID          string
	Core        string
	OS          string
	Arch        string
	Version     string // optional pin
	Timeout     time.Duration
}

type registryErrorEnvelope struct {
	Error registryAPIError `json:"error"`
}

type registryAPIError struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	Details   map[string]any `json:"details,omitempty"`
	Hint      string         `json:"hint,omitempty"`
	DocsURL   string         `json:"docsUrl,omitempty"`
	RequestID string         `json:"requestId"`
}

func Resolve(opts ResolveOptions) (*ResolveResponse, error) {
	reg := strings.TrimRight(strings.TrimSpace(opts.RegistryURL), "/")
	if reg == "" {
		if v := RegistryURLFromEnv(); v != "" {
			reg = strings.TrimRight(v, "/")
		} else {
			reg = defaultRegistryURL
		}
	}
	if strings.TrimSpace(opts.ID) == "" {
		return nil, fmt.Errorf("resolve: id required")
	}
	if strings.TrimSpace(opts.Core) == "" {
		return nil, fmt.Errorf("resolve: core required")
	}
	if strings.TrimSpace(opts.OS) == "" {
		opts.OS = runtime.GOOS
	}
	if strings.TrimSpace(opts.Arch) == "" {
		opts.Arch = runtime.GOARCH
	}

	u, err := url.Parse(reg)
	if err != nil {
		return nil, err
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/v1/extensions/resolve"
	q := u.Query()
	q.Set("id", opts.ID)
	q.Set("core", opts.Core)
	q.Set("os", opts.OS)
	q.Set("arch", opts.Arch)
	if strings.TrimSpace(opts.Version) != "" {
		q.Set("version", opts.Version)
	}
	u.RawQuery = q.Encode()

	client := &http.Client{Timeout: opts.Timeout}
	if client.Timeout == 0 {
		client.Timeout = 30 * time.Second
	}
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "runfabric-cli/"+opts.Core)
	token := strings.TrimSpace(opts.AuthToken)
	if token == "" {
		token = RegistryTokenFromEnv()
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		msg := strings.TrimSpace(string(b))
		var env registryErrorEnvelope
		if err := json.Unmarshal(b, &env); err == nil && strings.TrimSpace(env.Error.Code) != "" && strings.TrimSpace(env.Error.Message) != "" {
			parts := []string{fmt.Sprintf("resolve: %s: %s", env.Error.Code, env.Error.Message)}
			if strings.TrimSpace(env.Error.Hint) != "" {
				parts = append(parts, "hint: "+strings.TrimSpace(env.Error.Hint))
			}
			if strings.TrimSpace(env.Error.RequestID) != "" {
				parts = append(parts, "requestId: "+strings.TrimSpace(env.Error.RequestID))
			}
			// Use a constant format string to satisfy go vet.
			return nil, fmt.Errorf("%s", strings.Join(parts, " | "))
		}
		if msg == "" {
			msg = resp.Status
		}
		return nil, fmt.Errorf("resolve: %s", msg)
	}
	var rr ResolveResponse
	if err := json.NewDecoder(resp.Body).Decode(&rr); err != nil {
		return nil, err
	}
	return &rr, nil
}

type InstallFromRegistryOptions struct {
	RegistryURL string
	AuthToken   string
	ID          string
	Version     string // optional pin
	Timeout     time.Duration
}

type registryInstallReceipt struct {
	ReceiptVersion   int    `json:"version"`
	InstalledAt      string `json:"installedAt"`
	Kind             string `json:"kind"`
	ID               string `json:"id"`
	ExtensionVersion string `json:"extensionVersion"`

	RequestID string `json:"requestId"`

	Artifact struct {
		URL       string `json:"url"`
		SHA256    string `json:"sha256"`
		SizeBytes int64  `json:"sizeBytes"`
		Signature *struct {
			Algorithm   string `json:"algorithm"`
			Value       string `json:"value"`
			PublicKeyID string `json:"publicKeyId"`
		} `json:"signature,omitempty"`
	} `json:"artifact"`
}

// InstallFromRegistry resolves an extension and installs it locally.
// v1 supports plugins (provider/runtime/simulator/router/secret-manager/state) as external on-disk plugins.
func InstallFromRegistry(opts InstallFromRegistryOptions, coreVersion string) (*InstallResult, error) {
	rr, err := Resolve(ResolveOptions{
		RegistryURL: opts.RegistryURL,
		AuthToken:   opts.AuthToken,
		ID:          opts.ID,
		Core:        coreVersion,
		Version:     opts.Version,
		Timeout:     opts.Timeout,
	})
	if err != nil {
		return nil, err
	}
	var rp ResolvedPlugin
	if err := json.Unmarshal(rr.Resolved, &rp); err != nil {
		return nil, fmt.Errorf("resolve: unsupported resolved payload: %w", err)
	}
	requestID := ""
	if rr.Meta != nil {
		if v, ok := rr.Meta["requestId"].(string); ok {
			requestID = v
		}
	}
	if rp.Type != "plugin" {
		return nil, fmt.Errorf("install: resolved type %q not supported by extension install", rp.Type)
	}
	kind := manifests.NormalizePluginKind(rp.PluginKind)
	if !manifests.IsSupportedPluginKind(kind) {
		return nil, fmt.Errorf("install: unknown pluginKind %q", rp.PluginKind)
	}
	if rp.Artifact.Checksum.Algorithm != "sha256" || strings.TrimSpace(rp.Artifact.Checksum.Value) == "" {
		return nil, fmt.Errorf("install: registry artifact missing sha256 checksum")
	}
	if rp.Artifact.URL == "" {
		return nil, fmt.Errorf("install: registry artifact missing url")
	}

	home, err := HomeDir()
	if err != nil {
		return nil, err
	}
	cacheDir := filepath.Join(home, "cache")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, err
	}

	// Download binary into cache, verify sha256, then install into plugin dir with generated plugin.yaml.
	binPath, err := downloadToFile(cacheDir, rp.Artifact.URL, fmt.Sprintf("%s-%s-%s", rp.ID, rp.Version, filepath.Base(rp.Artifact.URL)), opts.Timeout)
	if err != nil {
		return nil, err
	}
	binBytes, err := os.ReadFile(binPath)
	if err != nil {
		return nil, err
	}
	// Verify checksum (sha256).
	wantHex := strings.TrimSpace(rp.Artifact.Checksum.Value)
	wantHex = strings.TrimPrefix(strings.ToLower(wantHex), "sha256:")
	sum := sha256.Sum256(binBytes)
	gotHex := hex.EncodeToString(sum[:])
	if !strings.EqualFold(gotHex, wantHex) {
		return nil, fmt.Errorf(
			"registry install checksum mismatch (requestId=%s). Hint: re-run install; if it persists, the registry response or artifact may be corrupted",
			requestID,
		)
	}
	// Verify signature if present. If missing, enforce signature only for verified publishers.
	if rp.Artifact.Signature != nil {
		if strings.TrimSpace(rp.Artifact.Signature.Algorithm) != "ed25519" {
			return nil, fmt.Errorf("registry install unsupported signature algorithm %q (requestId=%s)", rp.Artifact.Signature.Algorithm, requestID)
		}
		sigBytes, err := decodeSignatureValue(rp.Artifact.Signature.Value)
		if err != nil {
			return nil, fmt.Errorf("registry install signature decode failed (requestId=%s): %w", requestID, err)
		}
		pubKey, err := trustedPublicKey(rp.Artifact.Signature.PublicKeyID)
		if err != nil {
			return nil, fmt.Errorf("registry install unknown signature publicKeyId %q (requestId=%s)", rp.Artifact.Signature.PublicKeyID, requestID)
		}
		if !ed25519.Verify(pubKey, binBytes, sigBytes) {
			return nil, fmt.Errorf(
				"registry install signature verification failed (requestId=%s). Hint: the artifact bytes do not match the signature from the registry",
				requestID,
			)
		}
	} else {
		verifiedPublisher := false
		if rp.Publisher != nil {
			if v, ok := rp.Publisher["verified"].(bool); ok {
				verifiedPublisher = v
			}
		}
		if verifiedPublisher {
			return nil, fmt.Errorf(
				"registry install missing artifact signature for verified publisher (requestId=%s). Hint: ensure the registry provides an ed25519 artifact signature",
				requestID,
			)
		}
	}

	dest := pluginInstallDir(home, kind, rp.ID, rp.Version)
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return nil, err
	}

	binName := filepath.Base(rp.Artifact.URL)
	if binName == "" || binName == "." || binName == "/" {
		binName = "runfabric-plugin"
	}
	destBin := filepath.Join(dest, binName)
	if err := copyFile(binPath, destBin, 0o755); err != nil {
		return nil, err
	}

	// Persist local install receipt for traceability/auditing.
	receipt := &registryInstallReceipt{
		ReceiptVersion:   1,
		InstalledAt:      time.Now().UTC().Format(time.RFC3339),
		Kind:             string(kind),
		ID:               rp.ID,
		ExtensionVersion: rp.Version,
		RequestID:        requestID,
	}
	receipt.Artifact.URL = rp.Artifact.URL
	receipt.Artifact.SHA256 = rp.Artifact.Checksum.Value
	receipt.Artifact.SizeBytes = rp.Artifact.SizeBytes
	if rp.Artifact.Signature != nil {
		receipt.Artifact.Signature = &struct {
			Algorithm   string `json:"algorithm"`
			Value       string `json:"value"`
			PublicKeyID string `json:"publicKeyId"`
		}{
			Algorithm:   rp.Artifact.Signature.Algorithm,
			Value:       rp.Artifact.Signature.Value,
			PublicKeyID: rp.Artifact.Signature.PublicKeyID,
		}
	}
	if err := saveInstallReceipt(home, receipt); err != nil {
		return nil, err
	}

	py := pluginYAML{
		APIVersion:  "runfabric.io/plugin/v1",
		Kind:        string(kind),
		ID:          rp.ID,
		Name:        rp.Name,
		Description: rp.Description,
		Version:     rp.Version,
		PluginVer:   1,
		Executable:  binName,
	}
	// Conservative default: registry plugins are expected to need network/cloud and env in most cases.
	py.Permissions.Network = true
	py.Permissions.Cloud = true
	py.Permissions.Env = true

	yml, err := yaml.Marshal(py)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(dest, "plugin.yaml"), yml, 0o644); err != nil {
		return nil, err
	}
	// Also write checksums.txt for the binary we installed.
	_ = os.WriteFile(filepath.Join(dest, "checksums.txt"), []byte(fmt.Sprintf("%s  %s\n", strings.ToLower(rp.Artifact.Checksum.Value), binName)), 0o644)

	pm := &manifests.PluginManifest{
		ID:          rp.ID,
		Kind:        kind,
		Name:        rp.Name,
		Description: rp.Description,
		Source:      "external",
		Version:     rp.Version,
		Path:        dest,
		Executable:  destBin,
	}
	invalidateDiscoverCache()
	return &InstallResult{Plugin: pm}, nil
}

func decodeSignatureValue(v string) ([]byte, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil, fmt.Errorf("empty signature value")
	}
	// Common: base64.
	if b, err := base64.StdEncoding.DecodeString(v); err == nil {
		if len(b) == ed25519.SignatureSize {
			return b, nil
		}
	}
	if b, err := base64.RawStdEncoding.DecodeString(v); err == nil {
		if len(b) == ed25519.SignatureSize {
			return b, nil
		}
	}
	return nil, fmt.Errorf("signature value is not valid base64")
}

func trustedPublicKey(publicKeyID string) (ed25519.PublicKey, error) {
	switch strings.TrimSpace(publicKeyID) {
	case localDevPublicKeyID:
		pub, err := base64.StdEncoding.DecodeString(localDevPublicKeyB64)
		if err != nil {
			return nil, err
		}
		// ed25519 public key size is 32 bytes.
		if l := len(pub); l != ed25519.PublicKeySize {
			return nil, fmt.Errorf("unexpected public key size %d", l)
		}
		return ed25519.PublicKey(pub), nil
	default:
		return nil, fmt.Errorf("unknown publicKeyId")
	}
}

func saveInstallReceipt(home string, receipt *registryInstallReceipt) error {
	if receipt == nil {
		return fmt.Errorf("nil receipt")
	}
	receiptDir := filepath.Join(home, "registry-installs", receipt.Kind, receipt.ID, receipt.ExtensionVersion)
	if err := os.MkdirAll(receiptDir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(receiptDir, "receipt.json")
	b, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func downloadToFile(dir, srcURL, filename string, timeout time.Duration) (string, error) {
	if strings.TrimSpace(filename) == "" {
		filename = "download"
	}
	dest := filepath.Join(dir, filename)
	client := &http.Client{Timeout: timeout}
	if client.Timeout == 0 {
		client.Timeout = 60 * time.Second
	}
	resp, err := client.Get(srcURL) //nolint:gosec
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("download: %s", resp.Status)
	}
	f, err := os.Create(dest)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		return "", err
	}
	return dest, nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.WriteFile(dst, b, mode); err != nil {
		return err
	}
	return nil
}
