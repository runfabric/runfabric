package vercel

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// waitUntilDeploymentReady polls GET /v13/deployments/{id} until readyState == "READY".
// Budget: 60 attempts × 5s = 5 minutes.
func waitUntilDeploymentReady(ctx context.Context, deploymentID, teamID string) error {
	url := vercelAPI + "/v13/deployments/" + deploymentID
	if teamID != "" {
		url += "?teamId=" + teamID
	}
	for attempt := 0; attempt < 60; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+sdkprovider.Env("VERCEL_TOKEN"))
		resp, err := sdkprovider.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("poll deployment: %w", err)
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("poll deployment returned %s: %s", resp.Status, string(b))
		}
		var out struct {
			ReadyState string `json:"readyState"`
			State      string `json:"state"`
		}
		if err := json.Unmarshal(b, &out); err != nil {
			return fmt.Errorf("decode deployment: %w", err)
		}
		state := out.ReadyState
		if state == "" {
			state = out.State
		}
		switch state {
		case "READY":
			return nil
		case "ERROR", "CANCELED":
			return fmt.Errorf("vercel deployment entered terminal state %q", state)
		}
		if attempt < 59 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(5 * time.Second):
			}
		}
	}
	return fmt.Errorf("timed out waiting for vercel deployment %s to become ready", deploymentID)
}
