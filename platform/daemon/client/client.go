// Package client provides an HTTP client for the runfabricd daemon.
// When the daemon is running, the CLI delegates operations through it
// so the daemon acts as a central coordinator instead of each CLI
// invocation executing independently.
package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const (
	defaultPort    = 8766
	defaultTimeout = 30 * time.Second
	socketName     = "daemon.sock"
)

// Client speaks to a running runfabricd over HTTP (TCP or Unix socket).
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewFromSocket returns a Client that connects over the Unix socket at sockPath.
func NewFromSocket(sockPath string) *Client {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", sockPath)
		},
	}
	return &Client{
		baseURL:    "http://daemon",
		httpClient: &http.Client{Transport: transport, Timeout: defaultTimeout},
	}
}

// NewFromPort returns a Client that connects over TCP to host:port.
func NewFromPort(host string, port int) *Client {
	return &Client{
		baseURL:    fmt.Sprintf("http://%s:%d", host, port),
		httpClient: &http.Client{Timeout: defaultTimeout},
	}
}

// Discover returns a ready Client if the daemon is reachable, or nil.
// Prefers Unix socket (.runfabric/daemon.sock in cwd), falls back to TCP :8766.
func Discover() *Client {
	// Try Unix socket first — no TCP port required.
	cwd, err := os.Getwd()
	if err == nil {
		sockPath := filepath.Join(cwd, ".runfabric", socketName)
		if c := NewFromSocket(sockPath); c.ping() {
			return c
		}
	}
	// Fall back to TCP.
	c := NewFromPort("127.0.0.1", defaultPort)
	if c.ping() {
		return c
	}
	return nil
}

// IsRunning returns true if a daemon can be reached locally.
func IsRunning() bool { return Discover() != nil }

// Deploy forwards a deploy request to the daemon and returns the raw JSON response.
func (c *Client) Deploy(configPath, stage string) (json.RawMessage, error) {
	return c.post("/deploy", configPath, stage)
}

// Plan forwards a plan request to the daemon and returns the raw JSON response.
func (c *Client) Plan(configPath, stage string) (json.RawMessage, error) {
	return c.post("/plan", configPath, stage)
}

// Remove forwards a remove request to the daemon and returns the raw JSON response.
func (c *Client) Remove(configPath, stage string) (json.RawMessage, error) {
	return c.post("/remove", configPath, stage)
}

func (c *Client) ping() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/healthz", nil)
	if err != nil {
		return false
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func (c *Client) post(path, configPath, stage string) (json.RawMessage, error) {
	url := fmt.Sprintf("%s%s?stage=%s&config=%s", c.baseURL, path, stage, configPath)
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return nil, fmt.Errorf("daemon request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("daemon %s: %w", path, err)
	}
	defer resp.Body.Close()

	var body json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("daemon response decode: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("daemon %s status %d: %s", path, resp.StatusCode, body)
	}
	return body, nil
}
