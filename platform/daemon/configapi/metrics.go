package configapi

import (
	"time"

	"github.com/runfabric/runfabric/platform/observability/metrics"
)

const (
	metricOperationsTotal   = "runfabric_daemon_operations_total"
	metricOperationDuration = "runfabric_daemon_operation_duration_seconds"
	metricAuthRejections    = "runfabric_daemon_auth_rejections_total"
)

// operationLatencyBuckets cover config/deploy operations, which run seconds to
// minutes rather than the millisecond ranges of plain HTTP serving.
var operationLatencyBuckets = []float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60, 120, 300, 600}

// observeOperation records one config-API operation (validate/plan/deploy/...)
// with its outcome and duration.
func observeOperation(op string, start time.Time, err error) {
	outcome := "ok"
	if err != nil {
		outcome = "error"
	}
	metrics.Default.IncCounter(metricOperationsTotal,
		"Daemon config-API operations by operation and outcome.",
		map[string]string{"op": op, "outcome": outcome})
	metrics.Default.Observe(metricOperationDuration,
		"Daemon config-API operation duration in seconds.",
		operationLatencyBuckets,
		map[string]string{"op": op}, time.Since(start).Seconds())
}

// countAuthRejection records an authorization/rate-limit rejection.
func countAuthRejection(reason string) {
	metrics.Default.IncCounter(metricAuthRejections,
		"Daemon requests rejected before reaching a handler, by reason.",
		map[string]string{"reason": reason})
}
