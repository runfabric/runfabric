package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/runfabric/runfabric/internal/cli"
	"github.com/runfabric/runfabric/platform/observability/telemetry"
)

func main() {
	ctx := context.Background()
	if err := telemetry.Init(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "telemetry init: %v\n", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = telemetry.Shutdown(shutdownCtx)
	}()

	cmd := cli.NewRootCmd()
	cmd.Use = "runfabricd"

	args := os.Args[1:]
	if len(args) == 0 || args[0] != "daemon" {
		args = append([]string{"daemon"}, args...)
	}
	cmd.SetArgs(args)

	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
