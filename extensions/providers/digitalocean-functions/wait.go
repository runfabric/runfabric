package digitalocean

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// waitUntilAppActive polls GET /v2/apps/{id} until phase == "ACTIVE".
// Budget: 30 attempts × 10s = 5 minutes.
func waitUntilAppActive(ctx context.Context, appID string) error {
	url := doAPI + "/" + appID
	for attempt := 0; attempt < 30; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+sdkprovider.Env("DIGITALOCEAN_ACCESS_TOKEN"))
		resp, err := sdkprovider.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("poll app state: %w", err)
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("poll app state returned %s: %s", resp.Status, string(b))
		}
		var out struct {
			App struct {
				Phase string `json:"phase"`
			} `json:"app"`
		}
		if err := json.Unmarshal(b, &out); err != nil {
			return fmt.Errorf("decode app state: %w", err)
		}
		switch out.App.Phase {
		case "ACTIVE":
			return nil
		case "ERROR", "DELETED", "SUPERSEDED":
			return fmt.Errorf("digitalocean app entered terminal phase %q", out.App.Phase)
		}
		if attempt < 29 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(10 * time.Second):
			}
		}
	}
	return fmt.Errorf("timed out waiting for digitalocean app %s to become active", appID)
}
