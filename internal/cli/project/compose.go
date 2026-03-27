package project

import (
	"github.com/runfabric/runfabric/internal/app"
	"github.com/runfabric/runfabric/internal/cli/common"
	workflow "github.com/runfabric/runfabric/platform/workflow/core"
	"github.com/spf13/cobra"
)

func newComposeCmd(opts *GlobalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "compose",
		Short: "Multi-project compose operations",
		Long:  "Subcommands: plan, deploy, remove. Use -f runfabric.compose.yml (or default) for compose file.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newComposePlanCmd(opts), newComposeDeployCmd(opts), newComposeRemoveCmd(opts))
	return cmd
}

func newComposePlanCmd(opts *GlobalOptions) *cobra.Command {
	var composeFile string
	var concurrency int
	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Plan compose deployment",
		RunE: func(c *cobra.Command, args []string) error {
			_ = concurrency
			common.StatusRunning(opts.JSONOutput, "Planning compose...")
			comp, err := workflow.LoadCompose(composeFile)
			if err != nil {
				common.StatusFail(opts.JSONOutput, "Compose plan failed.")
				return common.PrintFailure("compose plan", err)
			}
			if _, err := workflow.ResolveServiceConfigPaths(composeFile, comp); err != nil {
				common.StatusFail(opts.JSONOutput, "Compose plan failed.")
				return common.PrintFailure("compose plan", err)
			}
			order, err := workflow.TopoOrder(comp)
			if err != nil {
				common.StatusFail(opts.JSONOutput, "Compose plan failed.")
				return common.PrintFailure("compose plan", err)
			}
			common.StatusDone(opts.JSONOutput, "Plan OK.")
			if opts.JSONOutput {
				return common.PrintJSONSuccess("compose plan", map[string]any{"order": order})
			}
			return common.PrintSuccess("compose plan", map[string]any{"order": order})
		},
	}
	cmd.Flags().StringVarP(&composeFile, "file", "f", "runfabric.compose.yml", "Compose file path")
	cmd.Flags().IntVar(&concurrency, "concurrency", 0, "Max concurrent deployments")
	return cmd
}

func newComposeDeployCmd(opts *GlobalOptions) *cobra.Command {
	var composeFile string
	var concurrency int
	var rollbackOnFailure, noRollbackOnFailure bool
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy compose projects",
		Long:  "Loads the compose file, deploys each service in dependency order, and injects SERVICE_*_URL from prior services' receipt outputs into dependent services.",
		RunE: func(c *cobra.Command, args []string) error {
			_ = concurrency
			common.StatusRunning(opts.JSONOutput, "Deploying compose...")
			result, err := app.ComposeDeploy(composeFile, opts.Stage, rollbackOnFailure, noRollbackOnFailure)
			if err != nil {
				common.StatusFail(opts.JSONOutput, "Compose deploy failed.")
				return common.PrintFailure("compose deploy", err)
			}
			common.StatusDone(opts.JSONOutput, "Compose deploy complete.")
			if opts.JSONOutput {
				return common.PrintJSONSuccess("compose deploy", result)
			}
			return common.PrintSuccess("compose deploy", result)
		},
	}
	cmd.Flags().StringVarP(&composeFile, "file", "f", "runfabric.compose.yml", "Compose file path")
	cmd.Flags().IntVar(&concurrency, "concurrency", 0, "Max concurrent deployments")
	cmd.Flags().BoolVar(&rollbackOnFailure, "rollback-on-failure", false, "Rollback on deploy failure")
	cmd.Flags().BoolVar(&noRollbackOnFailure, "no-rollback-on-failure", false, "Do not rollback on deploy failure")
	return cmd
}

func newComposeRemoveCmd(opts *GlobalOptions) *cobra.Command {
	var composeFile string
	var concurrency int
	var provider string
	cmd := &cobra.Command{
		Use:   "remove",
		Short: "Remove compose deployments",
		Long:  "Removes all services in the compose file in reverse dependency order.",
		RunE: func(c *cobra.Command, args []string) error {
			_ = concurrency
			_ = provider
			common.StatusRunning(opts.JSONOutput, "Removing compose...")
			result, err := app.ComposeRemove(composeFile, opts.Stage)
			if err != nil {
				common.StatusFail(opts.JSONOutput, "Compose remove failed.")
				return common.PrintFailure("compose remove", err)
			}
			common.StatusDone(opts.JSONOutput, "Compose remove complete.")
			if opts.JSONOutput {
				return common.PrintJSONSuccess("compose remove", result)
			}
			return common.PrintSuccess("compose remove", result)
		},
	}
	cmd.Flags().StringVarP(&composeFile, "file", "f", "runfabric.compose.yml", "Compose file path")
	cmd.Flags().IntVar(&concurrency, "concurrency", 0, "Max concurrent removals")
	cmd.Flags().StringVar(&provider, "provider", "", "Remove only this provider")
	return cmd
}
