package infrastructure

import (
	"github.com/runfabric/runfabric/internal/cli/common"
	"github.com/runfabric/runfabric/platform/workflow/app"
	"github.com/spf13/cobra"
)

func newBackendMigrateCmd(opts *common.GlobalOptions) *cobra.Command {
	var target string

	cmd := &cobra.Command{
		Use:   "backend-migrate",
		Short: "Migrate receipt and journal to another backend",
		RunE: func(cmd *cobra.Command, args []string) error {
			common.StatusRunning(opts.JSONOutput, "Migrating backend...")
			result, err := app.BackendMigrate(opts.ConfigPath, opts.Stage, target)
			if err != nil {
				common.StatusFail(opts.JSONOutput, "Backend migrate failed.")
				return common.PrintFailure("backend-migrate", err)
			}
			common.StatusDone(opts.JSONOutput, "Backend migrate complete.")
			if opts.JSONOutput {
				return common.PrintJSONSuccess("backend-migrate", result)
			}
			return common.PrintSuccess("backend-migrate", result)
		},
	}

	cmd.Flags().StringVar(&target, "target", "local", "Target backend kind: local|postgres|sqlite|s3|aws|dynamodb|gcs|azblob")
	return cmd
}
