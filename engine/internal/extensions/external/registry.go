package external

import (
	"crypto/sha256"
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

	"github.com/runfabric/runfabric/engine/internal/extensions/manifests"
	"gopkg.in/yaml.v3"
)

const (
	envRegistryURL     = "RUNFABRIC_REGISTRY_URL"
	envRegistryToken   = "RUNFABRIC_REGISTRY_TOKEN"
	defaultRegistryURL = "https://registry.runfabric.cloud"
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

// InstallFromRegistry resolves an extension and installs it locally.
// v1 supports only plugins (provider/runtime/simulator) as external on-disk plugins.
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
	if rp.Type != "plugin" {
		return nil, fmt.Errorf("install: resolved type %q not supported by extension install", rp.Type)
	}
	kind := manifests.PluginKind(strings.TrimSpace(rp.PluginKind))
	if kind != manifests.KindProvider && kind != manifests.KindRuntime && kind != manifests.KindSimulator {
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
	if err := verifySHA256File(binPath, rp.Artifact.Checksum.Value); err != nil {
		return nil, err
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
	return &InstallResult{Plugin: pm}, nil
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

func verifySHA256File(path, wantHex string) error {
	wantHex = strings.TrimSpace(wantHex)
	wantHex = strings.TrimPrefix(strings.ToLower(wantHex), "sha256:")
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(b)
	got := hex.EncodeToString(sum[:])
	if !strings.EqualFold(got, wantHex) {
		return fmt.Errorf("checksum mismatch for %s", filepath.Base(path))
	}
	return nil
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
