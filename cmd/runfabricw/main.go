package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	workercli "github.com/runfabric/runfabric/internal/cli/worker"
	"github.com/runfabric/runfabric/platform/observability/telemetry"
)

func main() {
	// Indirect through run so deferred telemetry shutdown (span flush) happens
	// before the process exits — os.Exit skips defers.
	os.Exit(run())
}

func run() int {
	ctx := context.Background()
	if err := telemetry.Init(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "telemetry init: %v\n", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = telemetry.Shutdown(shutdownCtx)
	}()

	// Join a caller's trace (TRACEPARENT/TRACESTATE env) so a worker run
	// spawned by an upstream service maps end-to-end in the tracing backend.
	ctx = telemetry.ContextFromEnv(ctx)
	ctx, end := telemetry.StartRootSpan(ctx, "runfabric/worker", spanName())

	err := workercli.NewRootCmd().ExecuteContext(ctx)
	end(err)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	return 0
}

// spanName names the worker span after the first subcommand (flags skipped).
func spanName() string {
	for _, arg := range os.Args[1:] {
		if !strings.HasPrefix(arg, "-") {
			return "runfabricw " + arg
		}
	}
	return "runfabricw"
}
