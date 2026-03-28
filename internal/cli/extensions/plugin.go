package extensions

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/runfabric/runfabric/internal/cli/common"
	manifests "github.com/runfabric/runfabric/platform/extensions/manifest"
	providerloader "github.com/runfabric/runfabric/platform/extensions/registry/loader/providers"
	"github.com/runfabric/runfabric/platform/workflow/app"
	"github.com/runfabric/runfabric/platform/workflow/lifecycle"
	"github.com/spf13/cobra"
)

func newPluginCmd(opts *common.GlobalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plugin",
		Short: "List and manage provider plugins",
		Long:  "Provider plugins (aws, gcp-functions, etc.). Use list, info, doctor, capabilities. enable/disable record preference in .runfabric/plugins.json.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(
		newPluginListCmd(opts),
		newPluginInfoCmd(opts),
		newPluginDoctorCmd(opts),
		newPluginEnableCmd(opts),
		newPluginDisableCmd(opts),
		newPluginCapabilitiesCmd(opts),
	)
	return cmd
}

func newPluginListCmd(opts *common.GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List provider plugins",
		RunE: func(c *cobra.Command, args []string) error {
			catalog, err := discoverPluginCatalog(false, false, nil)
			if err != nil {
				return err
			}
			list := catalog.Registry.List(manifests.KindProvider)
			if opts.JSONOutput {
				return writePrettyJSON(c.OutOrStdout(), map[string]any{"plugins": list})
			}
			for _, m := range list {
				fmt.Fprintf(c.OutOrStdout(), "%s\n", m.ID)
			}
			return nil
		},
	}
}

func newPluginInfoCmd(opts *common.GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "info [name]",
		Short: "Show plugin manifest for a provider",
		RunE: func(c *cobra.Command, args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("usage: runfabric plugin info <name>")
			}
			name := args[0]
			catalog, _ := discoverPluginCatalog(false, false, nil)
			if m := catalog.Registry.Get(name); m != nil && m.Kind == manifests.KindProvider {
				if opts.JSONOutput {
					return writePrettyJSON(c.OutOrStdout(), m)
				}
				renderPluginManifestInfo(c.OutOrStdout(), m)
				return nil
			}

			boundary, err := providerloader.LoadBoundary(providerloader.LoadOptions{IncludeExternal: true})
			if err != nil {
				return fmt.Errorf("load provider boundary: %w", err)
			}
			if p, ok := boundary.ProviderRegistry().Get(name); ok {
				meta := p.Meta()
				if opts.JSONOutput {
					return writePrettyJSON(c.OutOrStdout(), meta)
				}
				fmt.Fprintf(c.OutOrStdout(), "name:   %s\n", meta.Name)
				fmt.Fprintf(c.OutOrStdout(), "version: %s\n", meta.Version)
				fmt.Fprintf(c.OutOrStdout(), "capabilities: %v\n", meta.Capabilities)
				return nil
			}
			return fmt.Errorf("plugin %q not found", name)
		},
	}
}

func newPluginDoctorCmd(opts *common.GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor [name]",
		Short: "Run doctor for a provider (optional: provider name; default from config)",
		RunE: func(c *cobra.Command, args []string) error {
			ctx, err := app.Bootstrap(opts.ConfigPath, opts.Stage, "")
			if err != nil {
				return err
			}
			providerName := ctx.Config.Provider.Name
			if len(args) > 0 {
				providerName = args[0]
			}
			if _, ok := ctx.Registry.Get(providerName); !ok {
				return fmt.Errorf("plugin %q not found", providerName)
			}
			cfg := *ctx.Config
			cfg.Provider.Name = providerName
			result, err := lifecycle.Doctor(ctx.Registry, &cfg, opts.Stage)
			if err != nil {
				return err
			}
			if opts.JSONOutput {
				enc := json.NewEncoder(c.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}
			for _, check := range result.Checks {
				fmt.Fprintf(c.OutOrStdout(), "  %s\n", check)
			}
			return nil
		},
	}
}

func newPluginEnableCmd(opts *common.GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "enable [name]",
		Short: "Mark a plugin as enabled (record in .runfabric/plugins.json)",
		RunE: func(c *cobra.Command, args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("usage: runfabric plugin enable <name>")
			}
			return updatePluginsPref(args[0], true)
		},
	}
}

func newPluginDisableCmd(opts *common.GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "disable [name]",
		Short: "Mark a plugin as disabled (record in .runfabric/plugins.json)",
		RunE: func(c *cobra.Command, args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("usage: runfabric plugin disable <name>")
			}
			return updatePluginsPref(args[0], false)
		},
	}
}

func updatePluginsPref(name string, enabled bool) error {
	dir := filepath.Join(".runfabric")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, "plugins.json")
	var data map[string]any
	raw, _ := os.ReadFile(path)
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &data)
	}
	if data == nil {
		data = make(map[string]any)
	}
	disabled, _ := data["disabled"].([]any)
	disabledSet := make(map[string]bool)
	for _, n := range disabled {
		if s, ok := n.(string); ok {
			disabledSet[s] = true
		}
	}
	if enabled {
		delete(disabledSet, name)
	} else {
		disabledSet[name] = true
	}
	disabled = make([]any, 0, len(disabledSet))
	for n := range disabledSet {
		disabled = append(disabled, n)
	}
	data["disabled"] = disabled
	out, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}

func newPluginCapabilitiesCmd(opts *common.GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "capabilities [name]",
		Short: "Show plugin capabilities (runtimes, triggers, etc.)",
		RunE: func(c *cobra.Command, args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("usage: runfabric plugin capabilities <name>")
			}
			name := args[0]
			boundary, err := providerloader.LoadBoundary(providerloader.LoadOptions{IncludeExternal: true})
			if err != nil {
				return fmt.Errorf("load provider boundary: %w", err)
			}
			reg := boundary.ProviderRegistry()
			p, ok := reg.Get(name)
			if !ok {
				return fmt.Errorf("plugin %q not found", name)
			}
			meta := p.Meta()
			if opts.JSONOutput {
				return writePrettyJSON(c.OutOrStdout(), meta)
			}
			fmt.Fprintf(c.OutOrStdout(), "name:              %s\n", meta.Name)
			fmt.Fprintf(c.OutOrStdout(), "version:           %s\n", meta.Version)
			fmt.Fprintf(c.OutOrStdout(), "capabilities:      %v\n", meta.Capabilities)
			fmt.Fprintf(c.OutOrStdout(), "supportsRuntime:   %v\n", meta.SupportsRuntime)
			fmt.Fprintf(c.OutOrStdout(), "supportsTriggers:  %v\n", meta.SupportsTriggers)
			fmt.Fprintf(c.OutOrStdout(), "supportsResources: %v\n", meta.SupportsResources)
			return nil
		},
	}
}
