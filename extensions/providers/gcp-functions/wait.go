package gcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// waitUntilFunctionReady polls the GCP Long-Running Operation until it completes.
// GCP Cloud Functions v2 deploy returns an operation resource; the function is not
// invocable until the operation's done field is true with no error.
func waitUntilFunctionReady(ctx context.Context, operationName string) error {
	// Up to 60 attempts × 5s = 5 minutes.
	return retryWithBackoff(ctx, 60, 5*time.Second, func() error {
		url := gcpAPI + "/" + operationName
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+sdkprovider.Env("GCP_ACCESS_TOKEN"))
		resp, err := sdkprovider.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("poll operation: %w", err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("poll operation returned %s: %s", resp.Status, string(b))
		}
		var op struct {
			Done  bool `json:"done"`
			Error *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error,omitempty"`
		}
		if err := json.Unmarshal(b, &op); err != nil {
			return fmt.Errorf("decode operation: %w", err)
		}
		if op.Error != nil {
			return fmt.Errorf("gcp function deploy failed (code %d): %s", op.Error.Code, op.Error.Message)
		}
		if !op.Done {
			return fmt.Errorf("operation still in progress")
		}
		return nil
	})
}
