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
	"strings"
	"time"
)

// PublishFile describes one file in a registry publish session.
type PublishFile struct {
	Key       string `json:"key"`
	Name      string `json:"name"`
	SizeBytes int64  `json:"sizeBytes"`
	Checksum  struct {
		Algorithm string `json:"algorithm"`
		Value     string `json:"value"`
	} `json:"checksum"`
	LocalPath string `json:"-"`
}

// BuildPublishFileDescriptor computes file size + sha256 for publish init payloads.
func BuildPublishFileDescriptor(key, path string) (PublishFile, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return PublishFile{}, fmt.Errorf("publish file key is required")
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return PublishFile{}, fmt.Errorf("publish file path is required")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return PublishFile{}, err
	}
	sum := sha256.Sum256(b)
	var pf PublishFile
	pf.Key = key
	pf.Name = filepath.Base(path)
	pf.SizeBytes = int64(len(b))
	pf.Checksum.Algorithm = "sha256"
	pf.Checksum.Value = hex.EncodeToString(sum[:])
	pf.LocalPath = path
	return pf, nil
}

type PublishUpload struct {
	Key     string            `json:"key"`
	Method  string            `json:"method,omitempty"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
}

type PublishInitOptions struct {
	RegistryURL string
	AuthToken   string
	ID          string
	Version     string
	Type        string // addon | plugin
	PluginKind  string // provider | runtime | simulator | router | secret-manager | state (for type=plugin)
	Files       []PublishFile
	Timeout     time.Duration
}

type PublishInitResponse struct {
	PublishID string          `json:"publishId"`
	Status    string          `json:"status,omitempty"`
	Uploads   []PublishUpload `json:"uploads,omitempty"`
}

type PublishFinalizeOptions struct {
	RegistryURL string
	AuthToken   string
	PublishID   string
	Timeout     time.Duration
}

type PublishFinalizeResponse struct {
	PublishID string `json:"publishId"`
	Status    string `json:"status,omitempty"`
}

type PublishStatusOptions struct {
	RegistryURL string
	AuthToken   string
	PublishID   string
	Timeout     time.Duration
}

type PublishStatusResponse struct {
	PublishID string `json:"publishId"`
	Status    string `json:"status,omitempty"`
}

func PublishInit(opts PublishInitOptions) (*PublishInitResponse, error) {
	reg, err := normalizeRegistryURL(opts.RegistryURL)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(opts.ID) == "" {
		return nil, fmt.Errorf("publish init: id required")
	}
	if strings.TrimSpace(opts.Version) == "" {
		return nil, fmt.Errorf("publish init: version required")
	}
	typ := strings.TrimSpace(opts.Type)
	if typ == "" {
		typ = "plugin"
	}
	if typ != "plugin" && typ != "addon" {
		return nil, fmt.Errorf("publish init: type must be plugin or addon")
	}
	if len(opts.Files) == 0 {
		return nil, fmt.Errorf("publish init: at least one file is required")
	}
	body := map[string]any{
		"extension": map[string]any{
			"id":      strings.TrimSpace(opts.ID),
			"version": strings.TrimSpace(opts.Version),
			"type":    typ,
		},
		"files": opts.Files,
	}
	if typ == "plugin" && strings.TrimSpace(opts.PluginKind) != "" {
		body["extension"].(map[string]any)["pluginKind"] = strings.TrimSpace(opts.PluginKind)
	}
	u := strings.TrimRight(reg, "/") + "/v1/extensions/publish/init"
	b, err := registryJSONRequest(http.MethodPost, u, resolveRegistryToken(opts.AuthToken), body, opts.Timeout)
	if err != nil {
		return nil, err
	}
	var resp PublishInitResponse
	if err := json.Unmarshal(b, &resp); err != nil {
		return nil, err
	}
	if strings.TrimSpace(resp.PublishID) == "" {
		return nil, fmt.Errorf("publish init: missing publishId in response")
	}
	return &resp, nil
}

// UploadPublishFile uploads localPath to the signed upload URL returned by publish init.
func UploadPublishFile(upload PublishUpload, localPath string, timeout time.Duration) error {
	if strings.TrimSpace(upload.URL) == "" {
		return fmt.Errorf("publish upload: url required")
	}
	localPath = strings.TrimSpace(localPath)
	if localPath == "" {
		return fmt.Errorf("publish upload: local file path required")
	}
	f, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer f.Close()
	method := strings.ToUpper(strings.TrimSpace(upload.Method))
	if method == "" {
		method = http.MethodPut
	}
	req, err := http.NewRequest(method, upload.URL, f)
	if err != nil {
		return err
	}
	if _, ok := upload.Headers["Content-Type"]; !ok {
		req.Header.Set("Content-Type", "application/octet-stream")
	}
	for k, v := range upload.Headers {
		if strings.TrimSpace(k) == "" {
			continue
		}
		req.Header.Set(k, v)
	}
	client := &http.Client{Timeout: timeout}
	if client.Timeout == 0 {
		client.Timeout = 60 * time.Second
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := readBodyMessage(resp.Body)
		if msg == "" {
			msg = resp.Status
		}
		return fmt.Errorf("publish upload: %s", msg)
	}
	return nil
}

func PublishFinalize(opts PublishFinalizeOptions) (*PublishFinalizeResponse, error) {
	reg, err := normalizeRegistryURL(opts.RegistryURL)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(opts.PublishID) == "" {
		return nil, fmt.Errorf("publish finalize: publishId required")
	}
	u := strings.TrimRight(reg, "/") + "/v1/extensions/publish/finalize"
	b, err := registryJSONRequest(http.MethodPost, u, resolveRegistryToken(opts.AuthToken), map[string]any{
		"publishId": strings.TrimSpace(opts.PublishID),
	}, opts.Timeout)
	if err != nil {
		return nil, err
	}
	var resp PublishFinalizeResponse
	if err := json.Unmarshal(b, &resp); err != nil {
		return nil, err
	}
	if strings.TrimSpace(resp.PublishID) == "" {
		resp.PublishID = strings.TrimSpace(opts.PublishID)
	}
	return &resp, nil
}

func PublishStatus(opts PublishStatusOptions) (*PublishStatusResponse, error) {
	reg, err := normalizeRegistryURL(opts.RegistryURL)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(opts.PublishID) == "" {
		return nil, fmt.Errorf("publish status: publishId required")
	}
	u := strings.TrimRight(reg, "/") + "/v1/publish/" + url.PathEscape(strings.TrimSpace(opts.PublishID))
	b, err := registryJSONRequest(http.MethodGet, u, resolveRegistryToken(opts.AuthToken), nil, opts.Timeout)
	if err != nil {
		return nil, err
	}
	var resp PublishStatusResponse
	if err := json.Unmarshal(b, &resp); err != nil {
		return nil, err
	}
	if strings.TrimSpace(resp.PublishID) == "" {
		resp.PublishID = strings.TrimSpace(opts.PublishID)
	}
	return &resp, nil
}

func normalizeRegistryURL(in string) (string, error) {
	reg := strings.TrimRight(strings.TrimSpace(in), "/")
	if reg == "" {
		if v := RegistryURLFromEnv(); v != "" {
			reg = strings.TrimRight(v, "/")
		} else {
			reg = defaultRegistryURL
		}
	}
	if _, err := url.Parse(reg); err != nil {
		return "", err
	}
	return reg, nil
}

func resolveRegistryToken(in string) string {
	token := strings.TrimSpace(in)
	if token == "" {
		token = RegistryTokenFromEnv()
	}
	return token
}

func registryJSONRequest(method, u, token string, body any, timeout time.Duration) ([]byte, error) {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = strings.NewReader(string(b))
	}
	req, err := http.NewRequest(method, u, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	client := &http.Client{Timeout: timeout}
	if client.Timeout == 0 {
		client.Timeout = 30 * time.Second
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var env registryErrorEnvelope
		if err := json.Unmarshal(raw, &env); err == nil && strings.TrimSpace(env.Error.Code) != "" {
			msg := fmt.Sprintf("%s: %s", env.Error.Code, env.Error.Message)
			if strings.TrimSpace(env.Error.Hint) != "" {
				msg += " | hint: " + env.Error.Hint
			}
			if strings.TrimSpace(env.Error.RequestID) != "" {
				msg += " | requestId: " + env.Error.RequestID
			}
			return nil, fmt.Errorf("%s", msg)
		}
		msg := strings.TrimSpace(string(raw))
		if msg == "" {
			msg = resp.Status
		}
		return nil, fmt.Errorf("%s", msg)
	}
	return raw, nil
}

func readBodyMessage(r io.Reader) string {
	b, _ := io.ReadAll(io.LimitReader(r, 64*1024))
	return strings.TrimSpace(string(b))
}
