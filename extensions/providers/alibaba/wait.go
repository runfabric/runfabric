package alibaba

import (
	"context"
	"fmt"
	"time"
)

// waitUntilFunctionReady polls GetFunction until a 200 response is returned,
// indicating the function is active and invocable.
// Budget: 20 attempts × 3s = 1 minute.
func waitUntilFunctionReady(ctx context.Context, client *fcClient, serviceName, functionName string) error {
	path := "/" + fcAPIVersion + "/services/" + serviceName + "/functions/" + functionName
	for attempt := 0; attempt < 20; attempt++ {
		_, code, err := client.doSigned(ctx, "GET", path, nil, nil)
		if err != nil {
			return fmt.Errorf("poll function state: %w", err)
		}
		if code == 200 {
			return nil
		}
		if attempt < 19 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(3 * time.Second):
			}
		}
	}
	return fmt.Errorf("timed out waiting for alibaba function %s/%s to become active", serviceName, functionName)
}
