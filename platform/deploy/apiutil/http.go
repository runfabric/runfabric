// Package apiutil provides shared HTTP and env helpers for provider API calls.
package apiutil

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// Env returns os.Getenv(key).
func Env(key string) string { return os.Getenv(key) }

// DefaultClient is the default HTTP client for provider API calls.
var DefaultClient = &http.Client{Timeout: 5 * time.Minute}

// APIGet issues GET and decodes JSON into v. Auth: Bearer env(authEnv).
func APIGet(ctx context.Context, url, authEnv string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	if t := Env(authEnv); t != "" {
		req.Header.Set("Authorization", "Bearer "+t)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GET %s: %s: %s", url, resp.Status, string(body))
	}
	if v != nil {
		return json.NewDecoder(resp.Body).Decode(v)
	}
	return nil
}

// APIPost sends JSON body and decodes response into v.
func APIPost(ctx context.Context, url, authEnv string, body any, v any) error {
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			return err
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &buf)
	if err != nil {
		return err
	}
	if t := Env(authEnv); t != "" {
		req.Header.Set("Authorization", "Bearer "+t)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("POST %s: %s: %s", url, resp.Status, string(b))
	}
	if v != nil {
		return json.NewDecoder(resp.Body).Decode(v)
	}
	return nil
}

// APIPut sends body and returns response bytes.
func APIPut(ctx context.Context, url, authEnv string, body []byte, contentType string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if t := Env(authEnv); t != "" {
		req.Header.Set("Authorization", "Bearer "+t)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("PUT %s: %s: %s", url, resp.Status, string(b))
	}
	return b, nil
}

// DoDelete issues DELETE; 2xx or 204 accepted.
func DoDelete(ctx context.Context, url, authEnv string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	if t := Env(authEnv); t != "" {
		req.Header.Set("Authorization", "Bearer "+t)
	}
	resp, err := DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("DELETE %s: %s", url, resp.Status)
	}
	return nil
}
