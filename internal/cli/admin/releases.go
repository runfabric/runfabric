package admin

import (
	"github.com/runfabric/runfabric/internal/cli/common"
	"github.com/runfabric/runfabric/platform/workflow/app"
	"github.com/spf13/cobra"
)

func newReleasesCmd(opts *common.GlobalOptions) *cobra.Command {
	c := &cobra.Command{
		Use:   "releases",
		Short: "List deployment history (releases)",
		Long:  "Lists deployments (stages and timestamps) from the receipt backend. Same as runfabric deploy list.",
		RunE: func(cmd *cobra.Command, args []string) error {
			common.StatusRunning(opts.JSONOutput, "Listing releases...")
			result, err := app.Releases(opts.ConfigPath)
			if err != nil {
				common.StatusFail(opts.JSONOutput, "Releases failed.")
				return common.PrintFailure("releases", err)
			}
			common.StatusDone(opts.JSONOutput, "Releases complete.")
			if opts.JSONOutput {
				return common.PrintJSONSuccess("releases", result)
			}
			return common.PrintSuccess("releases", result)
		},
	}
	c.AddCommand(newReleaseHistoryCmd(opts))
	return c
}

// newReleaseHistoryCmd lists the retained past releases for a single stage — the
// engine snapshots each stage's receipt before a redeploy overwrites it, so this
// is the true per-stage deployment timeline (not just the current head).
func newReleaseHistoryCmd(opts *common.GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "history [stage]",
		Short: "Show retained past releases for a stage",
		Long:  "Lists the retained receipt snapshots for a stage, newest first. Stage may be given as an argument or via --stage.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			stage := opts.Stage
			if len(args) == 1 {
				stage = args[0]
			}
			common.StatusRunning(opts.JSONOutput, "Listing release history...")
			result, err := app.ReleaseHistory(opts.ConfigPath, stage)
			if err != nil {
				common.StatusFail(opts.JSONOutput, "Release history failed.")
				return common.PrintFailure("releases history", err)
			}
			common.StatusDone(opts.JSONOutput, "Release history complete.")
			if opts.JSONOutput {
				return common.PrintJSONSuccess("releases history", result)
			}
			return common.PrintSuccess("releases history", result)
		},
	}
}
