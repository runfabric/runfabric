// Package gcs implements a GCS-backed receipt state backend over the Cloud
// Storage JSON API. Auth matches the gcp-functions provider: GCP_ACCESS_TOKEN
// when set, else a GOOGLE_APPLICATION_CREDENTIALS service-account key
// exchanged (and auto-refreshed) by the plugin SDK's gcpauth.
package gcs

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/runfabric/runfabric/plugin-sdk/go/gcpauth"
)

const (
	defaultBaseURL   = "https://storage.googleapis.com/storage/v1"
	defaultUploadURL = "https://storage.googleapis.com/upload/storage/v1"
)

// Client is a minimal Cloud Storage JSON API client for state objects.
type Client struct {
	Bucket string
	Prefix string
	// BaseURL/UploadBaseURL are overridable for tests.
	BaseURL       string
	UploadBaseURL string
	HTTP          *http.Client
}

// New builds a client for bucket/prefix. Credentials: GCP_ACCESS_TOKEN or a
// GOOGLE_APPLICATION_CREDENTIALS service-account key (exchanged via gcpauth).
func New(bucket, prefix string) (*Client, error) {
	if strings.TrimSpace(bucket) == "" {
		return nil, fmt.Errorf("gcs state backend: bucket is required")
	}
	if err := gcpauth.EnsureAccessToken(context.Background()); err != nil {
		return nil, fmt.Errorf("gcs state backend: %w", err)
	}
	return &Client{
		Bucket:        bucket,
		Prefix:        strings.Trim(strings.TrimSpace(prefix), "/"),
		BaseURL:       defaultBaseURL,
		UploadBaseURL: defaultUploadURL,
		HTTP:          &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (c *Client) do(req *http.Request) (*http.Response, error) {
	// Re-ensure per request so SDK-minted tokens refresh before expiry in
	// long-running daemons; a user-provided token is never touched.
	_ = gcpauth.EnsureAccessToken(req.Context())
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(os.Getenv("GCP_ACCESS_TOKEN")))
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return client.Do(req)
}
