package cli

import (
	"github.com/runfabric/runfabric/engine/internal/app"
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

func newStateCmd(opts *GlobalOptions) *cobra.Command {
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
	)
	return cmd
}

func newStatePullCmd(opts *GlobalOptions) *cobra.Command {
	so := &stateOpts{}
	cmd := &cobra.Command{
		Use:   "pull",
		Short: "Pull state from remote backend",
		RunE: func(c *cobra.Command, args []string) error {
			_ = so
			statusRunning(opts.JSONOutput, "Pulling state...")
			result, err := app.StatePull(opts.ConfigPath, opts.Stage)
			if err != nil {
				statusFail(opts.JSONOutput, "State pull failed.")
				return printFailure("state pull", err)
			}
			statusDone(opts.JSONOutput, "State pull complete.")
			if opts.JSONOutput {
				return printJSONSuccess("state pull", result)
			}
			return printSuccess("state pull", result)
		},
	}
	cmd.Flags().StringVar(&so.Provider, "provider", "", "Provider name")
	cmd.Flags().StringVar(&so.Backend, "backend", "", stateBackendFlagHelp)
	return cmd
}

func newStateListCmd(opts *GlobalOptions) *cobra.Command {
	so := &stateOpts{}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List state entries",
		RunE: func(c *cobra.Command, args []string) error {
			_ = so
			statusRunning(opts.JSONOutput, "Listing state...")
			result, err := app.StateList(opts.ConfigPath, opts.Stage)
			if err != nil {
				statusFail(opts.JSONOutput, "State list failed.")
				return printFailure("state list", err)
			}
			statusDone(opts.JSONOutput, "State list complete.")
			if opts.JSONOutput {
				return printJSONSuccess("state list", result)
			}
			return printSuccess("state list", result)
		},
	}
	cmd.Flags().StringVar(&so.Provider, "provider", "", "Provider name")
	cmd.Flags().StringVar(&so.Backend, "backend", "", stateBackendFlagHelp)
	cmd.Flags().StringVar(&so.Service, "service", "", "Service name (default: from config)")
	return cmd
}

func newStateBackupCmd(opts *GlobalOptions) *cobra.Command {
	so := &stateOpts{}
	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Backup state to file",
		RunE: func(c *cobra.Command, args []string) error {
			statusRunning(opts.JSONOutput, "Backing up state...")
			result, err := app.StateBackup(opts.ConfigPath, opts.Stage, so.Out)
			if err != nil {
				statusFail(opts.JSONOutput, "State backup failed.")
				return printFailure("state backup", err)
			}
			statusDone(opts.JSONOutput, "State backup complete.")
			if opts.JSONOutput {
				return printJSONSuccess("state backup", result)
			}
			return printSuccess("state backup", result)
		},
	}
	cmd.Flags().StringVar(&so.Provider, "provider", "", "Provider name")
	cmd.Flags().StringVar(&so.Backend, "backend", "", stateBackendFlagHelp)
	cmd.Flags().StringVar(&so.Out, "out", "", "Output path for backup file")
	cmd.Flags().StringVar(&so.Service, "service", "", "Service name (default: from config)")
	return cmd
}

func newStateRestoreCmd(opts *GlobalOptions) *cobra.Command {
	so := &stateOpts{}
	cmd := &cobra.Command{
		Use:   "restore",
		Short: "Restore state from file",
		RunE: func(c *cobra.Command, args []string) error {
			statusRunning(opts.JSONOutput, "Restoring state...")
			result, err := app.StateRestore(opts.ConfigPath, opts.Stage, so.File)
			if err != nil {
				statusFail(opts.JSONOutput, "State restore failed.")
				return printFailure("state restore", err)
			}
			statusDone(opts.JSONOutput, "State restore complete.")
			if opts.JSONOutput {
				return printJSONSuccess("state restore", result)
			}
			return printSuccess("state restore", result)
		},
	}
	cmd.Flags().StringVar(&so.File, "file", "", "Path to backup file to restore")
	cmd.Flags().StringVar(&so.Backend, "backend", "", stateBackendFlagHelp)
	return cmd
}

func newStateForceUnlockCmd(opts *GlobalOptions) *cobra.Command {
	so := &stateOpts{}
	cmd := &cobra.Command{
		Use:   "force-unlock",
		Short: "Force-unlock a locked state",
		RunE: func(c *cobra.Command, args []string) error {
			_ = so
			statusRunning(opts.JSONOutput, "Force-unlocking...")
			result, err := app.StateForceUnlock(opts.ConfigPath, opts.Stage)
			if err != nil {
				statusFail(opts.JSONOutput, "State force-unlock failed.")
				return printFailure("state force-unlock", err)
			}
			statusDone(opts.JSONOutput, "State force-unlock complete.")
			if opts.JSONOutput {
				return printJSONSuccess("state force-unlock", result)
			}
			return printSuccess("state force-unlock", result)
		},
	}
	cmd.Flags().StringVar(&so.Provider, "provider", "", "Provider name")
	cmd.Flags().StringVar(&so.Backend, "backend", "", stateBackendFlagHelp)
	cmd.Flags().StringVar(&so.Service, "service", "", "Service name (default: from config)")
	return cmd
}

func newStateMigrateCmd(opts *GlobalOptions) *cobra.Command {
	so := &stateOpts{}
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Migrate state between backends",
		RunE: func(c *cobra.Command, args []string) error {
			statusRunning(opts.JSONOutput, "Migrating state...")
			result, err := app.StateMigrate(opts.ConfigPath, opts.Stage, so.From, so.To)
			if err != nil {
				statusFail(opts.JSONOutput, "State migrate failed.")
				return printFailure("state migrate", err)
			}
			statusDone(opts.JSONOutput, "State migrate complete.")
			if opts.JSONOutput {
				return printJSONSuccess("state migrate", result)
			}
			return printSuccess("state migrate", result)
		},
	}
	cmd.Flags().StringVar(&so.From, "from", "", "Source backend")
	cmd.Flags().StringVar(&so.To, "to", "", "Target backend")
	cmd.Flags().StringVar(&so.Provider, "provider", "", "Provider name")
	cmd.Flags().StringVar(&so.Backend, "backend", "", stateBackendFlagHelp)
	cmd.Flags().StringVar(&so.Service, "service", "", "Service name (default: from config)")
	return cmd
}

func newStateReconcileCmd(opts *GlobalOptions) *cobra.Command {
	so := &stateOpts{}
	cmd := &cobra.Command{
		Use:   "reconcile",
		Short: "Reconcile state with backend",
		RunE: func(c *cobra.Command, args []string) error {
			_ = so
			statusRunning(opts.JSONOutput, "Reconciling state...")
			result, err := app.StateReconcile(opts.ConfigPath, opts.Stage)
			if err != nil {
				statusFail(opts.JSONOutput, "State reconcile failed.")
				return printFailure("state reconcile", err)
			}
			statusDone(opts.JSONOutput, "State reconcile complete.")
			if opts.JSONOutput {
				return printJSONSuccess("state reconcile", result)
			}
			return printSuccess("state reconcile", result)
		},
	}
	cmd.Flags().StringVar(&so.Provider, "provider", "", "Provider name")
	cmd.Flags().StringVar(&so.Backend, "backend", "", stateBackendFlagHelp)
	cmd.Flags().StringVar(&so.Service, "service", "", "Service name (default: from config)")
	return cmd
}
