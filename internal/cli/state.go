package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newStateCmd(opts *GlobalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "state",
		Short: "State and lock operations (pull, list, backup, restore, force-unlock, migrate, reconcile)",
		Long:  "State commands manage backend state and locks. Use a subcommand: pull, list, backup, restore, force-unlock, migrate, reconcile.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	// Subcommands documented in command-reference; implementation pending (internal/state + backends).
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
	return &cobra.Command{
		Use:   "pull",
		Short: "Pull state from remote backend",
		RunE:  func(c *cobra.Command, args []string) error { return stateSubcommandNotImplemented("pull") },
	}
}

func newStateListCmd(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List state entries",
		RunE:  func(c *cobra.Command, args []string) error { return stateSubcommandNotImplemented("list") },
	}
}

func newStateBackupCmd(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "backup",
		Short: "Backup state to file",
		RunE:  func(c *cobra.Command, args []string) error { return stateSubcommandNotImplemented("backup") },
	}
}

func newStateRestoreCmd(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "restore",
		Short: "Restore state from file",
		RunE:  func(c *cobra.Command, args []string) error { return stateSubcommandNotImplemented("restore") },
	}
}

func newStateForceUnlockCmd(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "force-unlock",
		Short: "Force-unlock a locked state",
		RunE:  func(c *cobra.Command, args []string) error { return stateSubcommandNotImplemented("force-unlock") },
	}
}

func newStateMigrateCmd(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "migrate",
		Short: "Migrate state between backends",
		RunE:  func(c *cobra.Command, args []string) error { return stateSubcommandNotImplemented("migrate") },
	}
}

func newStateReconcileCmd(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "reconcile",
		Short: "Reconcile state with backend",
		RunE:  func(c *cobra.Command, args []string) error { return stateSubcommandNotImplemented("reconcile") },
	}
}

func stateSubcommandNotImplemented(name string) error {
	return fmt.Errorf("state %s: not yet implemented", name)
}
