package configuration

import (
	"fmt"
	"net/http"

	"github.com/runfabric/runfabric/platform/daemon/configapi"
	"github.com/spf13/cobra"
)

func newConfigAPICmd(opts *GlobalOptions) *cobra.Command {
	var address string
	var port int
	var apiKey string
	var rateLimit int

	cmd := &cobra.Command{
		Use:   "config-api",
		Short: "Run the YAML Configuration API server",
		Long:  "Serves POST /validate, POST /resolve, POST /plan, POST /deploy, POST /remove, POST /releases. Optional auth (--api-key) and rate limit (--rate-limit). Default: 0.0.0.0:8765.",
		RunE: func(cmd *cobra.Command, args []string) error {
			addr := fmt.Sprintf("%s:%d", address, port)
			srv := configapi.NewServer(opts.Stage)
			srv.APIKey = apiKey
			srv.RateLimitN = rateLimit
			server := &http.Server{
				Addr:    addr,
				Handler: srv.Handler(),
			}
			fmt.Printf("Config API listening on http://%s\n", addr)
			fmt.Println("  POST /validate, /resolve, /plan, /deploy, /remove, /releases — body: YAML, query: stage=dev")
			if apiKey != "" {
				fmt.Println("  Auth: X-API-Key required")
			}
			if rateLimit > 0 {
				fmt.Printf("  Rate limit: %d requests/min per client\n", rateLimit)
			}
			return server.ListenAndServe()
		},
	}

	cmd.Flags().StringVar(&address, "address", "0.0.0.0", "Listen address")
	cmd.Flags().IntVar(&port, "port", 8765, "Listen port")
	cmd.Flags().StringVar(&apiKey, "api-key", "", "Optional: require X-API-Key header (empty = no auth)")
	cmd.Flags().IntVar(&rateLimit, "rate-limit", 0, "Optional: max requests per minute per client (0 = disabled)")
	return cmd
}
