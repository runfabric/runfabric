package dynamodb

import (
	"context"
	"time"

	statetypes "github.com/runfabric/runfabric/extensions/types"
	"github.com/runfabric/runfabric/plugin-sdk/go/lease"
)

// StartHeartbeat keeps the handle's lease alive via the shared plugin-sdk
// lease primitive: renew every interval, stop and report on the first renewal
// failure, close the channel on exit.
func StartHeartbeat(ctx context.Context, handle *statetypes.Handle, leaseFor, interval time.Duration) <-chan error {
	if handle == nil {
		closed := make(chan error)
		close(closed)
		return closed
	}
	return lease.StartHeartbeat(ctx, handle.Renew, leaseFor, interval)
}
