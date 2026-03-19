package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/runfabric/runfabric/engine/internal/app"
	"github.com/runfabric/runfabric/engine/internal/extensions"
	"github.com/runfabric/runfabric/engine/internal/extensions/manifests"
	extproviders "github.com/runfabric/runfabric/engine/internal/extensions/providers"
	"github.com/runfabric/runfabric/engine/internal/extensions/resolution"
	"github.com/runfabric/runfabric/engine/internal/lifecycle"
	"github.com/spf13/cobra"
)

func newPluginCmd(opts *GlobalOptions) *cobra.Command {
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

func builtinProviderRegistry() *extproviders.Registry {
	b, err := resolution.NewCached(resolution.Options{IncludeExternal: false})
	if err != nil {
		// IncludeExternal=false should be deterministic; keep a safe fallback.
		reg := extensions.NewBuiltinProviderRegistry()
		resolution.RegisterAPIProviders(reg)
		return reg
	}
	return b.ProviderRegistry()
}

func newPluginListCmd(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List provider plugins",
		RunE: func(c *cobra.Command, args []string) error {
			reg := builtinProviderRegistry()
			list := reg.List()
			if opts.JSONOutput {
				enc := json.NewEncoder(c.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{"plugins": list})
			}
			for _, m := range list {
				fmt.Fprintf(c.OutOrStdout(), "%s\n", m.Name)
			}
			return nil
		},
	}
}

func newPluginInfoCmd(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "info [name]",
		Short: "Show plugin manifest for a provider",
		RunE: func(c *cobra.Command, args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("usage: runfabric plugin info <name>")
			}
			name := args[0]
			reg := manifests.NewPluginRegistry()
			m := reg.Get(name)
			if m == nil || m.Kind != manifests.KindProvider {
				// fallback: might be registered under different id
				reg2 := builtinProviderRegistry()
				if p, ok := reg2.GetPlugin(name); ok {
					meta := p.Meta()
					if opts.JSONOutput {
						enc := json.NewEncoder(c.OutOrStdout())
						enc.SetIndent("", "  ")
						return enc.Encode(meta)
					}
					fmt.Fprintf(c.OutOrStdout(), "name:   %s\n", meta.Name)
					fmt.Fprintf(c.OutOrStdout(), "version: %s\n", meta.Version)
					fmt.Fprintf(c.OutOrStdout(), "capabilities: %v\n", meta.Capabilities)
					return nil
				}
				return fmt.Errorf("plugin %q not found", name)
			}
			if opts.JSONOutput {
				enc := json.NewEncoder(c.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(m)
			}
			fmt.Fprintf(c.OutOrStdout(), "id:   %s\n", m.ID)
			fmt.Fprintf(c.OutOrStdout(), "kind: %s\n", m.Kind)
			fmt.Fprintf(c.OutOrStdout(), "name: %s\n", m.Name)
			fmt.Fprintf(c.OutOrStdout(), "description: %s\n", m.Description)
			return nil
		},
	}
}

func newPluginDoctorCmd(opts *GlobalOptions) *cobra.Command {
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
			if _, err := ctx.Registry.Get(providerName); err != nil {
				return err
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

func newPluginEnableCmd(opts *GlobalOptions) *cobra.Command {
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

func newPluginDisableCmd(opts *GlobalOptions) *cobra.Command {
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

func newPluginCapabilitiesCmd(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "capabilities [name]",
		Short: "Show plugin capabilities (runtimes, triggers, etc.)",
		RunE: func(c *cobra.Command, args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("usage: runfabric plugin capabilities <name>")
			}
			name := args[0]
			reg := builtinProviderRegistry()
			p, ok := reg.GetPlugin(name)
			if !ok {
				return fmt.Errorf("plugin %q not found", name)
			}
			meta := p.Meta()
			if opts.JSONOutput {
				enc := json.NewEncoder(c.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(meta)
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
