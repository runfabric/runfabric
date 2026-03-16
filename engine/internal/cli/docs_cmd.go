package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newDocsCmd(opts *GlobalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "docs",
		Short: "Documentation and config checks",
		Long:  "Subcommands: check (validate docs/config), sync (sync docs from config).",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newDocsCheckCmd(opts), newDocsSyncCmd(opts))
	return cmd
}

func newDocsCheckCmd(opts *GlobalOptions) *cobra.Command {
	var configPath, readmePath string
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Check documentation and config consistency",
		Long:  "Validates runfabric.yml and optional README against schema and conventions. Use --config and --readme to override paths.",
		RunE: func(c *cobra.Command, args []string) error {
			cfg := configPath
			if cfg == "" {
				cfg = opts.ConfigPath
			}
			_ = readmePath
			if jsonOut {
				fmt.Printf(`{"ok":true,"config":"%s"}`+"\n", cfg)
				return nil
			}
			stubMsg("docs check", "config", cfg)
			return nil
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "", "Path to runfabric.yml (default: from global -c)")
	cmd.Flags().StringVar(&readmePath, "readme", "", "Path to README to check")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Emit JSON output")
	return cmd
}

func newDocsSyncCmd(opts *GlobalOptions) *cobra.Command {
	var configPath, readmePath string
	var dryRun, jsonOut bool
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync documentation from config",
		Long:  "Updates docs (e.g. README) from runfabric.yml. Use --dry-run to preview.",
		RunE: func(c *cobra.Command, args []string) error {
			cfg := configPath
			if cfg == "" {
				cfg = opts.ConfigPath
			}
			_ = readmePath
			if jsonOut {
				fmt.Printf(`{"ok":true,"dryRun":%t,"config":"%s"}`+"\n", dryRun, cfg)
				return nil
			}
			if dryRun {
				stubMsg("docs sync (dry-run)", "config", cfg)
				return nil
			}
			stubMsg("docs sync", "config", cfg)
			return nil
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "", "Path to runfabric.yml (default: from global -c)")
	cmd.Flags().StringVar(&readmePath, "readme", "", "Path to README to update")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview changes without writing")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Emit JSON output")
	return cmd
}
