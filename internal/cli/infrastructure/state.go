package infrastructure

import (
	"github.com/runfabric/runfabric/internal/cli/common"
	"github.com/runfabric/runfabric/platform/workflow/app"
	"github.com/spf13/cobra"
)

// stateOpts holds flags shared by state subcommands (aligned with docs/COMMAND_REFERENCE.md).
type stateOpts struct {
	Provider string
	Backend  string
	Out      string
	File     string
	From     string
	To       string
	Service  string
}

const stateBackendFlagHelp = "Backend: local|postgres|sqlite|s3|aws|dynamodb|gcs|azblob"

func newStateCmd(opts *common.GlobalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "state",
		Short: "State and lock operations (pull, list, backup, restore, force-unlock, migrate, reconcile)",
		Long:  "State commands manage backend state and locks. Use a subcommand: pull, list, backup, restore, force-unlock, migrate, reconcile.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(
		newStatePullCmd(opts),
		newStateListCmd(opts),
		newStateBackupCmd(opts),
		newStateRestoreCmd(opts),
		newStateForceUnlockCmd(opts),
		newStateMigrateCmd(opts),
		newStateReconcileCmd(opts),
		newLockStealCmd(opts),
		newUnlockCmd(opts),
		newBackendMigrateCmd(opts),
	)
	return cmd
}

func newStatePullCmd(opts *common.GlobalOptions) *cobra.Command {
	so := &stateOpts{}
	cmd := &cobra.Command{
		Use:   "pull",
		Short: "Pull state from remote backend",
		RunE: func(c *cobra.Command, args []string) error {
			_ = so
			common.StatusRunning(opts.JSONOutput, "Pulling state...")
			result, err := app.StatePull(opts.ConfigPath, opts.Stage)
			if err != nil {
				common.StatusFail(opts.JSONOutput, "State pull failed.")
				return common.PrintFailure("state pull", err)
			}
			common.StatusDone(opts.JSONOutput, "State pull complete.")
			if opts.JSONOutput {
				return common.PrintJSONSuccess("state pull", result)
			}
			return common.PrintSuccess("state pull", result)
		},
	}
	cmd.Flags().StringVar(&so.Provider, "provider", "", "Provider name")
	cmd.Flags().StringVar(&so.Backend, "backend", "", stateBackendFlagHelp)
	return cmd
}

func newStateListCmd(opts *common.GlobalOptions) *cobra.Command {
	so := &stateOpts{}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List state entries",
		RunE: func(c *cobra.Command, args []string) error {
			_ = so
			common.StatusRunning(opts.JSONOutput, "Listing state...")
			result, err := app.StateList(opts.ConfigPath, opts.Stage)
			if err != nil {
				common.StatusFail(opts.JSONOutput, "State list failed.")
				return common.PrintFailure("state list", err)
			}
			common.StatusDone(opts.JSONOutput, "State list complete.")
			if opts.JSONOutput {
				return common.PrintJSONSuccess("state list", result)
			}
			return common.PrintSuccess("state list", result)
		},
	}
	cmd.Flags().StringVar(&so.Provider, "provider", "", "Provider name")
	cmd.Flags().StringVar(&so.Backend, "backend", "", stateBackendFlagHelp)
	cmd.Flags().StringVar(&so.Service, "service", "", "Service name (default: from config)")
	return cmd
}

func newStateBackupCmd(opts *common.GlobalOptions) *cobra.Command {
	so := &stateOpts{}
	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Backup state to file",
		RunE: func(c *cobra.Command, args []string) error {
			common.StatusRunning(opts.JSONOutput, "Backing up state...")
			result, err := app.StateBackup(opts.ConfigPath, opts.Stage, so.Out)
			if err != nil {
				common.StatusFail(opts.JSONOutput, "State backup failed.")
				return common.PrintFailure("state backup", err)
			}
			common.StatusDone(opts.JSONOutput, "State backup complete.")
			if opts.JSONOutput {
				return common.PrintJSONSuccess("state backup", result)
			}
			return common.PrintSuccess("state backup", result)
		},
	}
	cmd.Flags().StringVar(&so.Provider, "provider", "", "Provider name")
	cmd.Flags().StringVar(&so.Backend, "backend", "", stateBackendFlagHelp)
	cmd.Flags().StringVar(&so.Out, "out", "", "Output path for backup file")
	cmd.Flags().StringVar(&so.Service, "service", "", "Service name (default: from config)")
	return cmd
}

func newStateRestoreCmd(opts *common.GlobalOptions) *cobra.Command {
	so := &stateOpts{}
	cmd := &cobra.Command{
		Use:   "restore",
		Short: "Restore state from file",
		RunE: func(c *cobra.Command, args []string) error {
			common.StatusRunning(opts.JSONOutput, "Restoring state...")
			result, err := app.StateRestore(opts.ConfigPath, opts.Stage, so.File)
			if err != nil {
				common.StatusFail(opts.JSONOutput, "State restore failed.")
				return common.PrintFailure("state restore", err)
			}
			common.StatusDone(opts.JSONOutput, "State restore complete.")
			if opts.JSONOutput {
				return common.PrintJSONSuccess("state restore", result)
			}
			return common.PrintSuccess("state restore", result)
		},
	}
	cmd.Flags().StringVar(&so.File, "file", "", "Path to backup file to restore")
	cmd.Flags().StringVar(&so.Backend, "backend", "", stateBackendFlagHelp)
	return cmd
}

func newStateForceUnlockCmd(opts *common.GlobalOptions) *cobra.Command {
	so := &stateOpts{}
	cmd := &cobra.Command{
		Use:   "force-unlock",
		Short: "Force-unlock a locked state",
		RunE: func(c *cobra.Command, args []string) error {
			_ = so
			common.StatusRunning(opts.JSONOutput, "Force-unlocking...")
			result, err := app.StateForceUnlock(opts.ConfigPath, opts.Stage)
			if err != nil {
				common.StatusFail(opts.JSONOutput, "State force-unlock failed.")
				return common.PrintFailure("state force-unlock", err)
			}
			common.StatusDone(opts.JSONOutput, "State force-unlock complete.")
			if opts.JSONOutput {
				return common.PrintJSONSuccess("state force-unlock", result)
			}
			return common.PrintSuccess("state force-unlock", result)
		},
	}
	cmd.Flags().StringVar(&so.Provider, "provider", "", "Provider name")
	cmd.Flags().StringVar(&so.Backend, "backend", "", stateBackendFlagHelp)
	cmd.Flags().StringVar(&so.Service, "service", "", "Service name (default: from config)")
	return cmd
}

func newStateMigrateCmd(opts *common.GlobalOptions) *cobra.Command {
	so := &stateOpts{}
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Migrate state between backends",
		RunE: func(c *cobra.Command, args []string) error {
			common.StatusRunning(opts.JSONOutput, "Migrating state...")
			result, err := app.StateMigrate(opts.ConfigPath, opts.Stage, so.From, so.To)
			if err != nil {
				common.StatusFail(opts.JSONOutput, "State migrate failed.")
				return common.PrintFailure("state migrate", err)
			}
			common.StatusDone(opts.JSONOutput, "State migrate complete.")
			if opts.JSONOutput {
				return common.PrintJSONSuccess("state migrate", result)
			}
			return common.PrintSuccess("state migrate", result)
		},
	}
	cmd.Flags().StringVar(&so.From, "from", "", "Source backend")
	cmd.Flags().StringVar(&so.To, "to", "", "Target backend")
	cmd.Flags().StringVar(&so.Provider, "provider", "", "Provider name")
	cmd.Flags().StringVar(&so.Backend, "backend", "", stateBackendFlagHelp)
	cmd.Flags().StringVar(&so.Service, "service", "", "Service name (default: from config)")
	return cmd
}

func newStateReconcileCmd(opts *common.GlobalOptions) *cobra.Command {
	so := &stateOpts{}
	cmd := &cobra.Command{
		Use:   "reconcile",
		Short: "Reconcile state with backend",
		RunE: func(c *cobra.Command, args []string) error {
			_ = so
			common.StatusRunning(opts.JSONOutput, "Reconciling state...")
			result, err := app.StateReconcile(opts.ConfigPath, opts.Stage)
			if err != nil {
				common.StatusFail(opts.JSONOutput, "State reconcile failed.")
				return common.PrintFailure("state reconcile", err)
			}
			common.StatusDone(opts.JSONOutput, "State reconcile complete.")
			if opts.JSONOutput {
				return common.PrintJSONSuccess("state reconcile", result)
			}
			return common.PrintSuccess("state reconcile", result)
		},
	}
	cmd.Flags().StringVar(&so.Provider, "provider", "", "Provider name")
	cmd.Flags().StringVar(&so.Backend, "backend", "", stateBackendFlagHelp)
	cmd.Flags().StringVar(&so.Service, "service", "", "Service name (default: from config)")
	return cmd
}
