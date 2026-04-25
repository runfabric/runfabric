package fly

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// waitUntilMachineStarted polls GET /v1/apps/{app}/machines/{id} until state == "started".
// Budget: 30 attempts × 5s = 2.5 minutes.
func waitUntilMachineStarted(ctx context.Context, appName, machineID string) error {
	url := flyAPI + "/apps/" + appName + "/machines/" + machineID
	for attempt := 0; attempt < 30; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+sdkprovider.Env("FLY_API_TOKEN"))
		resp, err := sdkprovider.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("poll machine state: %w", err)
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("poll machine returned %s: %s", resp.Status, string(b))
		}
		var out struct {
			State string `json:"state"`
		}
		if err := json.Unmarshal(b, &out); err != nil {
			return fmt.Errorf("decode machine state: %w", err)
		}
		switch out.State {
		case "started":
			return nil
		case "failed", "destroyed":
			return fmt.Errorf("fly machine entered terminal state %q", out.State)
		}
		if attempt < 29 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(5 * time.Second):
			}
		}
	}
	return fmt.Errorf("timed out waiting for fly machine %s to start", machineID)
}
