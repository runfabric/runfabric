package common

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/runfabric/runfabric/internal/app"
	"github.com/runfabric/runfabric/platform/core/model/configpatch"
	"github.com/runfabric/runfabric/platform/extensions/registry/loader/runtime"
	"github.com/spf13/cobra"
)

func NewWorkflowCmd(opts *GlobalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workflow",
		Short: "Durable workflow runtime operations",
		Long:  "Workflow-first runtime operations: run, status, cancel, replay.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(
		newWorkflowRunCmd(opts),
		newWorkflowStatusCmd(opts),
		newWorkflowCancelCmd(opts),
		newWorkflowReplayCmd(opts),
	)
	return cmd
}

func newWorkflowRunCmd(opts *GlobalOptions) *cobra.Command {
	var workflowName string
	var runID string
	var inputRaw string
	var providerOverride string

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Start a workflow run",
		RunE: func(c *cobra.Command, args []string) error {
			cfgPath, err := resolveCLIConfigPath(opts.ConfigPath)
			if err != nil {
				if opts.JSONOutput {
					return WriteJSONEnvelope(c.OutOrStdout(), false, "workflow run", nil, &runtime.ErrorResponse{Code: "config_not_found", Message: err.Error()})
				}
				return err
			}
			runInput, err := parseWorkflowRunInput(inputRaw)
			if err != nil {
				if opts.JSONOutput {
					return WriteJSONEnvelope(c.OutOrStdout(), false, "workflow run", nil, &runtime.ErrorResponse{Code: "invalid_input", Message: err.Error()})
				}
				return err
			}

			StatusRunning(opts.JSONOutput, "Starting workflow run...")
			res, runErr := app.WorkflowRun(cfgPath, opts.Stage, providerOverride, workflowName, runID, runInput)
			if runErr != nil {
				StatusFail(opts.JSONOutput, "Workflow run failed.")
				if opts.JSONOutput {
					var result map[string]any
					if res != nil {
						result = map[string]any{
							"workflow": res.Workflow,
							"source":   res.Source,
							"warnings": res.Warnings,
							"run":      res.Run,
						}
					}
					return WriteJSONEnvelope(c.OutOrStdout(), false, "workflow run", result, &runtime.ErrorResponse{Code: "workflow_run_failed", Message: runErr.Error()})
				}
				return runErr
			}
			StatusDone(opts.JSONOutput, "Workflow run complete.")
			if opts.JSONOutput {
				return WriteJSONEnvelope(c.OutOrStdout(), true, "workflow run", map[string]any{
					"workflow": res.Workflow,
					"source":   res.Source,
					"warnings": res.Warnings,
					"run":      res.Run,
				}, nil)
			}
			return PrintSuccess("workflow run", map[string]any{
				"workflow": res.Workflow,
				"source":   res.Source,
				"warnings": res.Warnings,
				"run":      res.Run,
			})
		},
	}
	cmd.Flags().StringVar(&workflowName, "name", "", "Workflow name (defaults to the only configured workflow, when unambiguous)")
	cmd.Flags().StringVar(&runID, "run-id", "", "Optional run ID (auto-generated when omitted)")
	cmd.Flags().StringVar(&inputRaw, "input", "", "Optional JSON object to pass into the first step")
	cmd.Flags().StringVar(&providerOverride, "provider", "", "Provider key from providerOverrides (multi-cloud); e.g. aws, gcp")
	return cmd
}

func newWorkflowStatusCmd(opts *GlobalOptions) *cobra.Command {
	var runID string

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Get workflow run status",
		RunE: func(c *cobra.Command, args []string) error {
			cfgPath, err := resolveCLIConfigPath(opts.ConfigPath)
			if err != nil {
				if opts.JSONOutput {
					return WriteJSONEnvelope(c.OutOrStdout(), false, "workflow status", nil, &runtime.ErrorResponse{Code: "config_not_found", Message: err.Error()})
				}
				return err
			}
			StatusRunning(opts.JSONOutput, "Loading workflow run status...")
			run, err := app.WorkflowStatus(cfgPath, opts.Stage, runID)
			if err != nil {
				StatusFail(opts.JSONOutput, "Workflow status failed.")
				if opts.JSONOutput {
					return WriteJSONEnvelope(c.OutOrStdout(), false, "workflow status", nil, &runtime.ErrorResponse{Code: "workflow_status_failed", Message: err.Error()})
				}
				return err
			}
			StatusDone(opts.JSONOutput, "Workflow status loaded.")
			if opts.JSONOutput {
				return WriteJSONEnvelope(c.OutOrStdout(), true, "workflow status", map[string]any{"run": run}, nil)
			}
			return PrintSuccess("workflow status", map[string]any{"run": run})
		},
	}
	cmd.Flags().StringVar(&runID, "run-id", "", "Run ID")
	_ = cmd.MarkFlagRequired("run-id")
	return cmd
}

func newWorkflowCancelCmd(opts *GlobalOptions) *cobra.Command {
	var runID string

	cmd := &cobra.Command{
		Use:   "cancel",
		Short: "Request cancellation of a workflow run",
		RunE: func(c *cobra.Command, args []string) error {
			cfgPath, err := resolveCLIConfigPath(opts.ConfigPath)
			if err != nil {
				if opts.JSONOutput {
					return WriteJSONEnvelope(c.OutOrStdout(), false, "workflow cancel", nil, &runtime.ErrorResponse{Code: "config_not_found", Message: err.Error()})
				}
				return err
			}
			StatusRunning(opts.JSONOutput, "Requesting workflow cancellation...")
			run, err := app.WorkflowCancel(cfgPath, opts.Stage, runID)
			if err != nil {
				StatusFail(opts.JSONOutput, "Workflow cancellation failed.")
				if opts.JSONOutput {
					return WriteJSONEnvelope(c.OutOrStdout(), false, "workflow cancel", nil, &runtime.ErrorResponse{Code: "workflow_cancel_failed", Message: err.Error()})
				}
				return err
			}
			StatusDone(opts.JSONOutput, "Workflow cancellation requested.")
			if opts.JSONOutput {
				return WriteJSONEnvelope(c.OutOrStdout(), true, "workflow cancel", map[string]any{"run": run}, nil)
			}
			return PrintSuccess("workflow cancel", map[string]any{"run": run})
		},
	}
	cmd.Flags().StringVar(&runID, "run-id", "", "Run ID")
	_ = cmd.MarkFlagRequired("run-id")
	return cmd
}

func newWorkflowReplayCmd(opts *GlobalOptions) *cobra.Command {
	var runID string
	var fromStep string
	var providerOverride string

	cmd := &cobra.Command{
		Use:   "replay",
		Short: "Replay a workflow run from a specific step",
		RunE: func(c *cobra.Command, args []string) error {
			cfgPath, err := resolveCLIConfigPath(opts.ConfigPath)
			if err != nil {
				if opts.JSONOutput {
					return WriteJSONEnvelope(c.OutOrStdout(), false, "workflow replay", nil, &runtime.ErrorResponse{Code: "config_not_found", Message: err.Error()})
				}
				return err
			}
			StatusRunning(opts.JSONOutput, "Replaying workflow run...")
			run, err := app.WorkflowReplay(cfgPath, opts.Stage, providerOverride, runID, fromStep)
			if err != nil {
				StatusFail(opts.JSONOutput, "Workflow replay failed.")
				if opts.JSONOutput {
					return WriteJSONEnvelope(c.OutOrStdout(), false, "workflow replay", nil, &runtime.ErrorResponse{Code: "workflow_replay_failed", Message: err.Error()})
				}
				return err
			}
			StatusDone(opts.JSONOutput, "Workflow replay complete.")
			if opts.JSONOutput {
				return WriteJSONEnvelope(c.OutOrStdout(), true, "workflow replay", map[string]any{"run": run}, nil)
			}
			return PrintSuccess("workflow replay", map[string]any{"run": run})
		},
	}
	cmd.Flags().StringVar(&runID, "run-id", "", "Run ID")
	cmd.Flags().StringVar(&fromStep, "from-step", "", "Step ID to replay from")
	cmd.Flags().StringVar(&providerOverride, "provider", "", "Provider key from providerOverrides (multi-cloud); e.g. aws, gcp")
	_ = cmd.MarkFlagRequired("run-id")
	_ = cmd.MarkFlagRequired("from-step")
	return cmd
}

func resolveCLIConfigPath(configPath string) (string, error) {
	cwd, _ := os.Getwd()
	return configpatch.ResolveConfigPath(configPath, cwd, 5)
}

func parseWorkflowRunInput(raw string) (map[string]any, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		return nil, fmt.Errorf("--input must be a JSON object: %w", err)
	}
	if obj == nil {
		return nil, fmt.Errorf("--input must be a JSON object")
	}
	return obj, nil
}
