package azure

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// waitUntilAppReady polls the Azure ARM async operation URL (from Azure-AsyncOperation header)
// until the function app reaches Succeeded state. Falls back to polling the site resource
// directly if no async operation URL is present.
// Budget: 60 attempts × 5s = 5 minutes.
func waitUntilAppReady(ctx context.Context, asyncOpURL, siteURL string) error {
	if asyncOpURL != "" {
		return pollARMOperation(ctx, asyncOpURL)
	}
	return pollSiteState(ctx, siteURL)
}

func pollARMOperation(ctx context.Context, opURL string) error {
	return poll(ctx, 60, 5*time.Second, func() (bool, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, opURL, nil)
		if err != nil {
			return false, err
		}
		req.Header.Set("Authorization", "Bearer "+sdkprovider.Env("AZURE_ACCESS_TOKEN"))
		resp, err := sdkprovider.DefaultClient.Do(req)
		if err != nil {
			return false, fmt.Errorf("poll arm operation: %w", err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			return false, fmt.Errorf("poll arm operation returned %s: %s", resp.Status, string(b))
		}
		var op struct {
			Status string `json:"status"`
		}
		if err := json.Unmarshal(b, &op); err != nil {
			return false, fmt.Errorf("decode arm operation: %w", err)
		}
		switch op.Status {
		case "Succeeded":
			return true, nil
		case "Failed", "Canceled":
			return false, fmt.Errorf("azure function app provisioning %s", op.Status)
		default:
			return false, nil // still in progress
		}
	})
}

func pollSiteState(ctx context.Context, siteURL string) error {
	return poll(ctx, 60, 5*time.Second, func() (bool, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, siteURL, nil)
		if err != nil {
			return false, err
		}
		req.Header.Set("Authorization", "Bearer "+sdkprovider.Env("AZURE_ACCESS_TOKEN"))
		resp, err := sdkprovider.DefaultClient.Do(req)
		if err != nil {
			return false, fmt.Errorf("poll site state: %w", err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		if resp.StatusCode == http.StatusNotFound {
			return false, nil // still provisioning
		}
		if resp.StatusCode != http.StatusOK {
			return false, fmt.Errorf("poll site state returned %s: %s", resp.Status, string(b))
		}
		var site struct {
			Properties struct {
				State string `json:"state"`
			} `json:"properties"`
		}
		if err := json.Unmarshal(b, &site); err != nil {
			return false, fmt.Errorf("decode site state: %w", err)
		}
		if site.Properties.State == "Running" {
			return true, nil
		}
		return false, nil
	})
}

func poll(ctx context.Context, attempts int, delay time.Duration, fn func() (bool, error)) error {
	for i := 0; i < attempts; i++ {
		done, err := fn()
		if err != nil {
			return err
		}
		if done {
			return nil
		}
		if i < attempts-1 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}
	}
	return fmt.Errorf("timed out waiting for azure function app to become ready")
}
