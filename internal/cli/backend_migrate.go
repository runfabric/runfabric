package cli

import (
	"github.com/runfabric/runfabric/internal/app"
	"github.com/spf13/cobra"
)

func newBackendMigrateCmd(opts *GlobalOptions) *cobra.Command {
	var target string

	cmd := &cobra.Command{
		Use:   "backend-migrate",
		Short: "Migrate receipt and journal to another backend",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := app.BackendMigrate(opts.ConfigPath, opts.Stage, target)
			if err != nil {
				return printFailure("backend-migrate", err)
			}
			if opts.JSONOutput {
				return printJSONSuccess("backend-migrate", result)
			}
			return printSuccess("backend-migrate", result)
		},
	}

	cmd.Flags().StringVar(&target, "target", "local", "Target backend kind")
	return cmd
}
