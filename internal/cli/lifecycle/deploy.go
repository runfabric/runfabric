package lifecycle

import (
	"encoding/json"
	"fmt"

	"github.com/runfabric/runfabric/internal/cli/common"
	daemonclient "github.com/runfabric/runfabric/platform/daemon/client"
	"github.com/runfabric/runfabric/platform/workflow/app"
	"github.com/spf13/cobra"
)

// runDeploy runs app.Deploy and prints result; used by deploy, deploy fn, deploy function, deploy-function.
// If preview is non-empty, it is used as the stage (e.g. --preview pr-123 => stage pr-123 for preview environments).
// providerOverride is the key from providerOverrides in runfabric.yml (e.g. aws, gcp) for multi-cloud; use "" for single provider.
// When the local daemon is running and no per-function or override flags are set, the request is proxied through it.
func runDeploy(opts *common.GlobalOptions, functionName string, rollbackOnFailure, noRollbackOnFailure bool, preview, providerOverride, label string) error {
	stage := opts.Stage
	if preview != "" {
		stage = preview
	}

	// Proxy through the daemon when it is running and no flags require direct execution.
	if functionName == "" && !rollbackOnFailure && !noRollbackOnFailure && providerOverride == "" {
		if dc := daemonclient.Discover(); dc != nil {
			common.StatusRunning(opts.JSONOutput, "Deploying via daemon...")
			raw, err := dc.Deploy(opts.ConfigPath, stage)
			if err != nil {
				common.StatusFail(opts.JSONOutput, "Deploy failed.")
				return common.PrintFailure(label, fmt.Errorf("daemon deploy: %w", err))
			}
			common.StatusDone(opts.JSONOutput, "Deploy complete.")
			if opts.JSONOutput {
				return common.PrintJSONSuccess(label, json.RawMessage(raw))
			}
			return common.PrintSuccess(label, json.RawMessage(raw))
		}
	}

	service := resolveAppService(opts)
	common.StatusRunning(opts.JSONOutput, "Deploying...")
	result, err := service.Deploy(opts.ConfigPath, stage, functionName, rollbackOnFailure, noRollbackOnFailure, nil, providerOverride)
	if err != nil {
		common.StatusFail(opts.JSONOutput, "Deploy failed.")
		return common.PrintFailure(label, err)
	}
	common.StatusDone(opts.JSONOutput, "Deploy complete.")
	if opts.JSONOutput {
		return common.PrintJSONSuccess(label, result)
	}
	return common.PrintSuccess(label, result)
}

func newDeployCmd(opts *common.GlobalOptions) *cobra.Command {
	var function, outDir, preview, sourceURL, providerOverride string
	var rollbackOnFailure, noRollbackOnFailure bool

	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy the service",
		Long:  "Deploy the service (all functions or a single function with --function). Use --preview <id> for preview environments (e.g. pr-123). Use --source <url> to deploy from an archive URL (e.g. GitHub tarball); use -c/--config <path> to supply runfabric.yml from outside the source (code from URL, config from file). Use --provider <key> when runfabric.yml has providerOverrides for multi-cloud (e.g. --provider aws --stage prod). Rollback precedence: CLI --rollback-on-failure/--no-rollback-on-failure, then runfabric.yml deploy.rollbackOnFailure, then RUNFABRIC_ROLLBACK_ON_FAILURE.",
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = outDir
			stage := opts.Stage
			if preview != "" {
				stage = preview
			}
			if sourceURL != "" {
				common.StatusRunning(opts.JSONOutput, "Deploying...")
				result, err := app.DeployFromSourceURL(
					opts.ConfigPath,
					sourceURL,
					stage,
					function,
					rollbackOnFailure,
					noRollbackOnFailure,
					providerOverride,
				)
				if err != nil {
					common.StatusFail(opts.JSONOutput, "Deploy from source failed.")
					return common.PrintFailure("deploy", err)
				}
				common.StatusDone(opts.JSONOutput, "Deploy complete.")
				if opts.JSONOutput {
					return common.PrintJSONSuccess("deploy", result)
				}
				return common.PrintSuccess("deploy", result)
			}
			return runDeploy(opts, function, rollbackOnFailure, noRollbackOnFailure, preview, providerOverride, "deploy")
		},
	}

	cmd.Flags().StringVarP(&function, "function", "f", "", "Deploy only this function (default: all)")
	cmd.Flags().StringVar(&outDir, "out", "", "Output directory for artifacts")
	cmd.Flags().StringVar(&preview, "preview", "", "Preview environment id (e.g. pr-123); uses this as stage for isolated deploy")
	cmd.Flags().StringVar(&sourceURL, "source", "", "Deploy from archive URL (e.g. https://github.com/org/repo/archive/main.zip)")
	cmd.Flags().StringVar(&providerOverride, "provider", "", "Provider key from providerOverrides (multi-cloud); e.g. aws, gcp")
	cmd.Flags().BoolVar(&rollbackOnFailure, "rollback-on-failure", false, "Rollback on deploy failure")
	cmd.Flags().BoolVar(&noRollbackOnFailure, "no-rollback-on-failure", false, "Do not rollback on deploy failure")

	cmd.AddCommand(newDeployFnCmd(opts), newDeployFunctionCmd(opts), newDeployListCmd(opts))
	return cmd
}

func newDeployFnCmd(opts *common.GlobalOptions) *cobra.Command {
	var outDir string
	var rollbackOnFailure, noRollbackOnFailure bool
	cmd := &cobra.Command{
		Use:   "fn [name]",
		Short: "Deploy a single function by name",
		RunE: func(c *cobra.Command, args []string) error {
			name := ""
			if len(args) > 0 {
				name = args[0]
			}
			_ = outDir
			return runDeploy(opts, name, rollbackOnFailure, noRollbackOnFailure, "", "", "deploy fn")
		},
	}
	cmd.Flags().StringVar(&outDir, "out", "", "Output directory for artifacts")
	cmd.Flags().BoolVar(&rollbackOnFailure, "rollback-on-failure", false, "Rollback on deploy failure")
	cmd.Flags().BoolVar(&noRollbackOnFailure, "no-rollback-on-failure", false, "Do not rollback on deploy failure")
	return cmd
}

func newDeployFunctionCmd(opts *common.GlobalOptions) *cobra.Command {
	var outDir string
	var rollbackOnFailure, noRollbackOnFailure bool
	cmd := &cobra.Command{
		Use:   "function [name]",
		Short: "Deploy a single function by name",
		RunE: func(c *cobra.Command, args []string) error {
			name := ""
			if len(args) > 0 {
				name = args[0]
			}
			_ = outDir
			return runDeploy(opts, name, rollbackOnFailure, noRollbackOnFailure, "", "", "deploy function")
		},
	}
	cmd.Flags().StringVar(&outDir, "out", "", "Output directory for artifacts")
	cmd.Flags().BoolVar(&rollbackOnFailure, "rollback-on-failure", false, "Rollback on deploy failure")
	cmd.Flags().BoolVar(&noRollbackOnFailure, "no-rollback-on-failure", false, "Do not rollback on deploy failure")
	return cmd
}

func newDeployFunctionStandaloneCmd(opts *common.GlobalOptions) *cobra.Command {
	var outDir string
	var rollbackOnFailure, noRollbackOnFailure bool
	cmd := &cobra.Command{
		Use:   "deploy-function [name]",
		Short: "Deploy a single function by name",
		RunE: func(c *cobra.Command, args []string) error {
			name := ""
			if len(args) > 0 {
				name = args[0]
			}
			_ = outDir
			return runDeploy(opts, name, rollbackOnFailure, noRollbackOnFailure, "", "", "deploy-function")
		},
	}
	cmd.Flags().StringVar(&outDir, "out", "", "Output directory for artifacts")
	cmd.Flags().BoolVar(&rollbackOnFailure, "rollback-on-failure", false, "Rollback on deploy failure")
	cmd.Flags().BoolVar(&noRollbackOnFailure, "no-rollback-on-failure", false, "Do not rollback on deploy failure")
	return cmd
}

func newDeployListCmd(opts *common.GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List deployments (releases) for the service",
		Long:  "Lists deployment history (stages and timestamps) from the receipt backend.",
		RunE: func(cmd *cobra.Command, args []string) error {
			common.StatusRunning(opts.JSONOutput, "Listing releases...")
			result, err := app.Releases(opts.ConfigPath)
			if err != nil {
				common.StatusFail(opts.JSONOutput, "Releases failed.")
				return common.PrintFailure("deploy list", err)
			}
			common.StatusDone(opts.JSONOutput, "List complete.")
			if opts.JSONOutput {
				return common.PrintJSONSuccess("deploy list", result)
			}
			return common.PrintSuccess("deploy list", result)
		},
	}
}
