package admin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/runfabric/runfabric/platform/core/model/config"
	"github.com/runfabric/runfabric/platform/core/model/configpatch"
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
		Long:  "Validates runfabric.yml and optional README. Use --config and --readme to override paths.",
		RunE: func(c *cobra.Command, args []string) error {
			cwd, _ := os.Getwd()
			cfgPath := configPath
			if cfgPath == "" {
				cfgPath = opts.ConfigPath
			}
			resolved, resolveErr := configpatch.ResolveConfigPath(cfgPath, cwd, 5)
			var err error
			var valid bool
			if resolveErr == nil && resolved != "" {
				cfg, loadErr := config.Load(resolved)
				if loadErr != nil {
					err = loadErr
				} else {
					err = config.Validate(cfg)
					valid = err == nil
				}
			} else {
				if resolveErr != nil {
					err = resolveErr
				} else {
					err = fmt.Errorf("no runfabric.yml found")
				}
			}
			readmeOK := true
			if readmePath != "" {
				if _, statErr := os.Stat(readmePath); statErr != nil {
					readmeOK = false
					if err == nil {
						err = statErr
					}
				}
			}
			if jsonOut {
				out := map[string]any{
					"ok":      valid && readmeOK,
					"command": "docs check",
					"config":  resolved,
					"valid":   valid,
				}
				if err != nil {
					out["error"] = err.Error()
				}
				if readmePath != "" {
					out["readmeOk"] = readmeOK
				}
				enc := json.NewEncoder(c.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}
			if err != nil {
				return err
			}
			fmt.Fprintf(c.OutOrStdout(), "docs check: config valid, config=%s\n", resolved)
			if readmePath != "" && !readmeOK {
				fmt.Fprintf(c.OutOrStderr(), "readme %s not found\n", readmePath)
			}
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
		Long:  "Updates README from runfabric.yml (e.g. service name). Use --dry-run to preview.",
		RunE: func(c *cobra.Command, args []string) error {
			cwd, _ := os.Getwd()
			cfgPath := configPath
			if cfgPath == "" {
				cfgPath = opts.ConfigPath
			}
			resolved, resolveErr := configpatch.ResolveConfigPath(cfgPath, cwd, 5)
			if resolveErr != nil || resolved == "" {
				errMsg := "no runfabric.yml found"
				if resolveErr != nil {
					errMsg = resolveErr.Error()
				}
				if jsonOut {
					enc := json.NewEncoder(c.OutOrStdout())
					return enc.Encode(map[string]any{"ok": false, "command": "docs sync", "error": errMsg})
				}
				return fmt.Errorf("%s", errMsg)
			}
			cfg, err := config.Load(resolved)
			if err != nil {
				if jsonOut {
					enc := json.NewEncoder(c.OutOrStdout())
					return enc.Encode(map[string]any{"ok": false, "command": "docs sync", "error": err.Error()})
				}
				return err
			}
			if err := config.Validate(cfg); err != nil {
				if jsonOut {
					enc := json.NewEncoder(c.OutOrStdout())
					return enc.Encode(map[string]any{"ok": false, "command": "docs sync", "error": err.Error()})
				}
				return err
			}
			projectRoot := filepath.Dir(resolved)
			readmeFile := readmePath
			if readmeFile == "" {
				readmeFile = filepath.Join(projectRoot, "README.md")
			}
			block := fmt.Sprintf("\n\n## RunFabric\n\n- Service: %s\n- Provider: %s\n", cfg.Service, cfg.Provider.Name)
			if dryRun {
				if jsonOut {
					enc := json.NewEncoder(c.OutOrStdout())
					return enc.Encode(map[string]any{
						"ok": true, "command": "docs sync", "dryRun": true,
						"readme": readmeFile, "wouldAppend": true,
					})
				}
				fmt.Fprintf(c.OutOrStdout(), "docs sync (dry-run): would append to %s:\n%s", readmeFile, block)
				return nil
			}
			existing, _ := os.ReadFile(readmeFile)
			content := strings.TrimSpace(string(existing))
			if strings.Contains(content, "## RunFabric") {
				if jsonOut {
					enc := json.NewEncoder(c.OutOrStdout())
					return enc.Encode(map[string]any{"ok": true, "command": "docs sync", "readme": readmeFile, "updated": false})
				}
				fmt.Fprintf(c.OutOrStdout(), "docs sync: %s already has RunFabric section\n", readmeFile)
				return nil
			}
			newContent := content + block + "\n"
			if err := os.WriteFile(readmeFile, []byte(newContent), 0o644); err != nil {
				if jsonOut {
					enc := json.NewEncoder(c.OutOrStdout())
					return enc.Encode(map[string]any{"ok": false, "command": "docs sync", "error": err.Error()})
				}
				return err
			}
			if jsonOut {
				enc := json.NewEncoder(c.OutOrStdout())
				return enc.Encode(map[string]any{"ok": true, "command": "docs sync", "readme": readmeFile, "updated": true})
			}
			fmt.Fprintf(c.OutOrStdout(), "docs sync: updated %s\n", readmeFile)
			return nil
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "", "Path to runfabric.yml (default: from global -c)")
	cmd.Flags().StringVar(&readmePath, "readme", "", "Path to README to update")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview changes without writing")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Emit JSON output")
	return cmd
}
