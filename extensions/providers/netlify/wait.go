package netlify

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// waitUntilDeployReady polls GET /api/v1/deploys/{id} until state == "ready".
// Budget: 60 attempts × 5s = 5 minutes.
func waitUntilDeployReady(ctx context.Context, deployID string) error {
	url := netlifyAPI + "/deploys/" + deployID
	for attempt := 0; attempt < 60; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+sdkprovider.Env("NETLIFY_AUTH_TOKEN"))
		resp, err := sdkprovider.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("poll deploy: %w", err)
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("poll deploy returned %s: %s", resp.Status, string(b))
		}
		var out struct {
			State string `json:"state"`
		}
		if err := json.Unmarshal(b, &out); err != nil {
			return fmt.Errorf("decode deploy state: %w", err)
		}
		switch out.State {
		case "ready":
			return nil
		case "error":
			return fmt.Errorf("netlify deploy %s failed", deployID)
		}
		if attempt < 59 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(5 * time.Second):
			}
		}
	}
	return fmt.Errorf("timed out waiting for netlify deploy %s to become ready", deployID)
}
