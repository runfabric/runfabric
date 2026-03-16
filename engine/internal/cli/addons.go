package cli

import (
	"encoding/json"
	"fmt"

	"github.com/runfabric/runfabric/engine/internal/config"
	"github.com/spf13/cobra"
)

func newAddonsCmd(opts *GlobalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "addons",
		Short: "List or validate add-ons",
		Long:  "Add-ons are declared in runfabric.yml under 'addons'. Use addons list to see the built-in catalog. Addon secrets are bound at deploy and injected into function environment.",
	}
	cmd.AddCommand(newAddonsListCmd(opts))
	return cmd
}

func newAddonsListCmd(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List add-ons in the catalog",
		RunE: func(cmd *cobra.Command, args []string) error {
			catalog := config.AddonCatalog()
			if opts.ConfigPath != "" {
				cfg, err := config.Load(opts.ConfigPath)
				if err == nil && cfg.AddonCatalogURL != "" {
					fetched := config.FetchAddonCatalog(cfg.AddonCatalogURL)
					if len(fetched) > 0 {
						catalog = config.MergeAddonCatalogs(catalog, fetched)
					}
				}
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
