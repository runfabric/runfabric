package extensions

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/runfabric/runfabric/platform/core/model/config"
	"github.com/runfabric/runfabric/platform/core/model/configpatch"
	"github.com/spf13/cobra"
)

func newAddonsCmd(opts *GlobalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "addons",
		Short: "List or validate add-ons",
		Long:  "RunFabric Addons: function/app-level augmentation. Declared in runfabric.yml under 'addons'; use 'runfabric extensions addons list' as the catalog view. Addon secrets are bound at deploy and injected into function environment. See also 'runfabric extensions extension list' for plugins (providers, runtimes).",
	}
	cmd.AddCommand(newAddonsListCmd(opts), newAddonsValidateCmd(opts), newAddonsAddCmd(opts), newAddonsRemoveCmd(opts))
	return cmd
}

func newAddonsListCmd(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List add-ons in the catalog",
		RunE: func(cmd *cobra.Command, args []string) error {
			var addonCatalogURL string
			if opts.ConfigPath != "" {
				cfg, err := config.Load(opts.ConfigPath)
				if err == nil && cfg.AddonCatalogURL != "" {
					addonCatalogURL = cfg.AddonCatalogURL
				}
			}
			catalog := config.AddonCatalog()
			fetched := config.FetchAddonCatalog(addonCatalogURL)
			if len(fetched) > 0 {
				catalog = config.MergeAddonCatalogs(catalog, fetched)
			}
			if opts.JSONOutput {
				out := map[string]any{"addons": catalog}
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}
			for _, e := range catalog {
				if e.Description != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "%s — %s\n", e.Name, e.Description)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "%s\n", e.Name)
				}
			}
			return nil
		},
	}
}

func newAddonsValidateCmd(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "validate [addon-id]",
		Short: "Validate runfabric.yml and optional addon (e.g. sentry)",
		RunE: func(c *cobra.Command, args []string) error {
			cwd, _ := os.Getwd()
			cfgPath, err := configpatch.ResolveConfigPath(opts.ConfigPath, cwd, 5)
			if err != nil {
				return err
			}
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return err
			}
			if err := config.Validate(cfg); err != nil {
				if opts.JSONOutput {
					_ = json.NewEncoder(c.OutOrStdout()).Encode(map[string]any{"ok": false, "error": err.Error()})
					return err
				}
				return err
			}
			if len(args) > 0 {
				addonID := args[0]
				if cfg.Addons != nil {
					if _, ok := cfg.Addons[addonID]; !ok {
						// check if it's attached to a function
						found := false
						for _, fn := range cfg.Functions {
							for _, a := range fn.Addons {
								if a == addonID {
									found = true
									break
								}
							}
						}
						if !found {
							return fmt.Errorf("addon %q not declared in addons or functions.*.addons", addonID)
						}
					}
				} else {
					return fmt.Errorf("addon %q not declared (no addons block in config)", addonID)
				}
			}
			if opts.JSONOutput {
				_ = json.NewEncoder(c.OutOrStdout()).Encode(map[string]any{"ok": true})
				return nil
			}
			if len(args) > 0 {
				fmt.Fprintf(c.OutOrStdout(), "addon validate: %s ok\n", args[0])
			} else {
				fmt.Fprintln(c.OutOrStdout(), "addon validate: ok")
			}
			return nil
		},
	}
}

func newAddonsAddCmd(opts *GlobalOptions) *cobra.Command {
	var function string
	cmd := &cobra.Command{
		Use:   "add [addon-id]",
		Short: "Add an addon to a function (patches runfabric.yml)",
		RunE: func(c *cobra.Command, args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("usage: runfabric addons add <addon-id> --function <name>")
			}
			if function == "" {
				return fmt.Errorf("--function is required")
			}
			addonID := args[0]
			cwd, _ := os.Getwd()
			cfgPath, err := configpatch.ResolveConfigPath(opts.ConfigPath, cwd, 5)
			if err != nil {
				return err
			}
			if err := configpatch.AppendFunctionAddon(cfgPath, function, addonID); err != nil {
				return err
			}
			if opts.JSONOutput {
				_ = json.NewEncoder(c.OutOrStdout()).Encode(map[string]any{"ok": true, "addon": addonID, "function": function})
				return nil
			}
			fmt.Fprintf(c.OutOrStdout(), "addon %q added to function %q\n", addonID, function)
			return nil
		},
	}
	cmd.Flags().StringVar(&function, "function", "", "Function name to attach the addon to")
	return cmd
}

func newAddonsRemoveCmd(opts *GlobalOptions) *cobra.Command {
	var function string
	cmd := &cobra.Command{
		Use:   "remove [addon-id]",
		Short: "Remove an addon from a function (patches runfabric.yml)",
		RunE: func(c *cobra.Command, args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("usage: runfabric extensions addons remove <addon-id> --function <name>")
			}
			if function == "" {
				return fmt.Errorf("--function is required")
			}
			addonID := args[0]
			cwd, _ := os.Getwd()
			cfgPath, err := configpatch.ResolveConfigPath(opts.ConfigPath, cwd, 5)
			if err != nil {
				return err
			}
			if err := configpatch.RemoveFunctionAddon(cfgPath, function, addonID); err != nil {
				return err
			}
			if opts.JSONOutput {
				_ = json.NewEncoder(c.OutOrStdout()).Encode(map[string]any{"ok": true, "addon": addonID, "function": function})
				return nil
			}
			fmt.Fprintf(c.OutOrStdout(), "addon %q removed from function %q\n", addonID, function)
			return nil
		},
	}
	cmd.Flags().StringVar(&function, "function", "", "Function name to remove the addon from")
	return cmd
}
