package configuration

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/runfabric/runfabric/internal/cli/common"
	"github.com/runfabric/runfabric/platform/daemon/configapi"
	daemonserver "github.com/runfabric/runfabric/platform/daemon/server"
	"github.com/spf13/cobra"
)

func newConfigAPICmd(opts *common.GlobalOptions) *cobra.Command {
	var address string
	var port int
	var apiKey string
	var rateLimit int

	cmd := &cobra.Command{
		Use:   "config-api",
		Short: "Run the YAML Configuration API server",
		Long:  "Serves POST /validate, POST /resolve, POST /plan, POST /deploy, POST /remove, POST /releases. Optional auth (--api-key) and rate limit (--rate-limit). Default: 0.0.0.0:8765.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := daemonserver.RequireAuthForBind(address, apiKey); err != nil {
				return err
			}
			addr := fmt.Sprintf("%s:%d", address, port)
			srv := configapi.NewServer(opts.Stage)
			srv.APIKey = apiKey
			srv.RateLimitN = rateLimit
			server := &http.Server{
				Addr:              addr,
				Handler:           srv.Handler(),
				ReadHeaderTimeout: 10 * time.Second,
				ReadTimeout:       30 * time.Second,
				WriteTimeout:      120 * time.Second,
				IdleTimeout:       120 * time.Second,
			}
			fmt.Printf("Config API listening on http://%s\n", addr)
			fmt.Println("  POST /validate, /resolve, /plan, /deploy, /remove, /releases — body: YAML, query: stage=dev")
			if apiKey != "" {
				fmt.Println("  Auth: X-API-Key required")
			}
			if rateLimit > 0 {
				fmt.Printf("  Rate limit: %d requests/min per client\n", rateLimit)
			}

			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
			serveErr := make(chan error, 1)
			go func() {
				err := server.ListenAndServe()
				if err == http.ErrServerClosed {
					err = nil
				}
				serveErr <- err
			}()
			select {
			case err := <-serveErr:
				return err
			case <-sigCh:
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
				defer cancel()
				return server.Shutdown(shutdownCtx)
			}
		},
	}

	cmd.Flags().StringVar(&address, "address", "127.0.0.1", "Listen address (default loopback; binding a non-loopback address requires --api-key)")
	cmd.Flags().IntVar(&port, "port", 8765, "Listen port")
	cmd.Flags().StringVar(&apiKey, "api-key", "", "Optional: require X-API-Key header (empty = no auth)")
	cmd.Flags().IntVar(&rateLimit, "rate-limit", 0, "Optional: max requests per minute per client (0 = disabled)")
	return cmd
}
