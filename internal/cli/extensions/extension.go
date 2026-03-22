package extensions

import (
	"encoding/json"
	"fmt"
	"text/tabwriter"

	"github.com/runfabric/runfabric/internal/cli/common"
	"github.com/runfabric/runfabric/platform/extensions/application/external"
	manifests "github.com/runfabric/runfabric/platform/extensions/manifest"
	extRuntime "github.com/runfabric/runfabric/platform/extensions/registry/loader/runtime"
	"github.com/runfabric/runfabric/platform/extensions/registry/resolution"
	"github.com/spf13/cobra"
)

func newExtensionCmd(opts *GlobalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "extension",
		Short: "List or inspect RunFabric plugins (Phase 15)",
		Long:  "RunFabric Extensions: plugins (providers, runtimes, simulators) and addons. Use 'runfabric extensions extension list' to see built-in + installed external plugins; 'runfabric extensions addons list' for the addon catalog.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(
		newExtensionListCmd(opts),
		newExtensionInfoCmd(opts),
		newExtensionSearchCmd(opts),
		newExtensionInstallCmd(opts),
		newExtensionUninstallCmd(opts),
		newExtensionUpgradeCmd(opts),
	)
	return cmd
}

func newExtensionInstallCmd(opts *GlobalOptions) *cobra.Command {
	var kind string
	var version string
	var source string
	var registry string
	var registryToken string
	cmd := &cobra.Command{
		Use:   "install <id>",
		Short: "Install an extension from a URL or local archive",
		Long:  "Installs an external plugin into RUNFABRIC_HOME/plugins/<kind>/<id>/<version>/. You can install from --source (URL/path) or from a registry (default https://registry.runfabric.cloud).",
		RunE: func(c *cobra.Command, args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("usage: runfabric extension install <id> [--source <url|path>] [--registry <url>] [--kind provider|runtime|simulator] [--version v]")
			}
			id := args[0]
			rc := common.LoadRunfabricrc()
			if registry == "" && external.RegistryURLFromEnv() == "" && rc.RegistryURL != "" {
				registry = rc.RegistryURL
			}
			if registryToken == "" && external.RegistryTokenFromEnv() == "" && rc.RegistryToken != "" {
				registryToken = rc.RegistryToken
			}
			if registryToken == "" && external.RegistryTokenFromEnv() == "" && rc.RegistryToken == "" {
				tok, terr := common.RegistryTokenFromAuthStore(c.Context(), "")
				if terr != nil {
					return terr
				}
				registryToken = tok
			}
			var res any
			var err error
			if source != "" {
				parsedKind, perr := parsePluginKindFlag(kind)
				if perr != nil {
					return perr
				}
				res, err = external.Install(external.InstallOptions{
					ID:          id,
					Kind:        parsedKind,
					Version:     version,
					Source:      source,
					RegistryURL: registry,
					AuthToken:   registryToken,
					CoreVersion: extRuntime.Version,
				})
			} else {
				// Registry path: resolve + download + install.
				r, ierr := external.InstallFromRegistry(
					external.InstallFromRegistryOptions{
						RegistryURL: registry,
						AuthToken:   registryToken,
						ID:          id,
						Version:     version,
					},
					extRuntime.Version,
				)
				res, err = r, ierr
			}
			if err != nil {
				return err
			}
			if opts.JSONOutput {
				enc := json.NewEncoder(c.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(res)
			}
			// Text output: best-effort for both install paths.
			if ir, ok := res.(*external.InstallResult); ok && ir != nil && ir.Plugin != nil {
				fmt.Fprintf(c.OutOrStdout(), "Installed %s (%s) %s\n", ir.Plugin.ID, ir.Plugin.Kind, ir.Plugin.Version)
				fmt.Fprintf(c.OutOrStdout(), "Path: %s\n", ir.Plugin.Path)
			} else {
				fmt.Fprintln(c.OutOrStdout(), "Installed.")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&kind, "kind", "", "Plugin kind: provider, runtime, simulator (required when using --source)")
	cmd.Flags().StringVar(&version, "version", "", "Expected version (optional; best-effort validation)")
	cmd.Flags().StringVar(&source, "source", "", "Source archive URL or local file path (.zip/.tar.gz). If omitted, installs via registry resolve.")
	cmd.Flags().StringVar(&registry, "registry", "", "Registry base URL (default: https://registry.runfabric.cloud; override via RUNFABRIC_REGISTRY_URL)")
	cmd.Flags().StringVar(&registryToken, "registry-token", "", "Registry bearer token (override via RUNFABRIC_REGISTRY_TOKEN or .runfabricrc registry.token)")
	return cmd
}

func newExtensionUninstallCmd(opts *GlobalOptions) *cobra.Command {
	var kind string
	var version string
	cmd := &cobra.Command{
		Use:   "uninstall <id>",
		Short: "Uninstall an installed extension",
		RunE: func(c *cobra.Command, args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("usage: runfabric extension uninstall <id> [--kind provider|runtime|simulator] [--version v]")
			}
			id := args[0]
			var k manifests.PluginKind
			if kind != "" {
				parsedKind, perr := parsePluginKindFlag(kind)
				if perr != nil {
					return perr
				}
				k = parsedKind
			}
			if err := external.Uninstall(external.UninstallOptions{ID: id, Kind: k, Version: version}); err != nil {
				return err
			}
			if opts.JSONOutput {
				_, _ = c.OutOrStdout().Write([]byte(`{"ok":true}` + "\n"))
				return nil
			}
			fmt.Fprintf(c.OutOrStdout(), "Uninstalled %s\n", id)
			return nil
		},
	}
	cmd.Flags().StringVar(&kind, "kind", "", "Plugin kind: provider, runtime, simulator (optional)")
	cmd.Flags().StringVar(&version, "version", "", "Remove a specific version (optional)")
	return cmd
}

func newExtensionUpgradeCmd(opts *GlobalOptions) *cobra.Command {
	var kind string
	var source string
	var registry string
	var registryToken string
	cmd := &cobra.Command{
		Use:   "upgrade <id>",
		Short: "Upgrade an extension (reinstall from source)",
		Long:  "Upgrade reinstalls an external plugin. Use --source (URL/path) or resolve from a registry (default https://registry.runfabric.cloud).",
		RunE: func(c *cobra.Command, args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("usage: runfabric extension upgrade <id> [--source <url|path>] [--registry <url>] [--kind provider|runtime|simulator]")
			}
			id := args[0]
			rc := common.LoadRunfabricrc()
			if registry == "" && external.RegistryURLFromEnv() == "" && rc.RegistryURL != "" {
				registry = rc.RegistryURL
			}
			if registryToken == "" && external.RegistryTokenFromEnv() == "" && rc.RegistryToken != "" {
				registryToken = rc.RegistryToken
			}
			if registryToken == "" && external.RegistryTokenFromEnv() == "" && rc.RegistryToken == "" {
				tok, terr := common.RegistryTokenFromAuthStore(c.Context(), "")
				if terr != nil {
					return terr
				}
				registryToken = tok
			}
			var res any
			var err error
			if source != "" {
				parsedKind, perr := parsePluginKindFlag(kind)
				if perr != nil {
					return perr
				}
				res, err = external.Install(external.InstallOptions{
					ID:          id,
					Kind:        parsedKind,
					Source:      source,
					RegistryURL: registry,
					AuthToken:   registryToken,
					CoreVersion: extRuntime.Version,
				})
			} else {
				r, ierr := external.InstallFromRegistry(
					external.InstallFromRegistryOptions{
						RegistryURL: registry,
						AuthToken:   registryToken,
						ID:          id,
					},
					extRuntime.Version,
				)
				res, err = r, ierr
			}
			if err != nil {
				return err
			}
			if opts.JSONOutput {
				enc := json.NewEncoder(c.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(res)
			}
			if ir, ok := res.(*external.InstallResult); ok && ir != nil && ir.Plugin != nil {
				fmt.Fprintf(c.OutOrStdout(), "Upgraded %s (%s) to %s\n", ir.Plugin.ID, ir.Plugin.Kind, ir.Plugin.Version)
			} else {
				fmt.Fprintln(c.OutOrStdout(), "Upgraded.")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&kind, "kind", "", "Plugin kind: provider, runtime, simulator (required when using --source)")
	cmd.Flags().StringVar(&source, "source", "", "Source archive URL or local file path (.zip/.tar.gz). If omitted, upgrades via registry resolve.")
	cmd.Flags().StringVar(&registry, "registry", "", "Registry base URL (default: https://registry.runfabric.cloud; override via RUNFABRIC_REGISTRY_URL)")
	cmd.Flags().StringVar(&registryToken, "registry-token", "", "Registry bearer token (override via RUNFABRIC_REGISTRY_TOKEN or .runfabricrc registry.token)")
	return cmd
}

func newExtensionListCmd(opts *GlobalOptions) *cobra.Command {
	var kind string
	var showInvalid bool
	var preferExternal bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List RunFabric plugins (providers, runtimes, simulators)",
		RunE: func(c *cobra.Command, args []string) error {
			prefer := preferExternal || external.PreferExternalFromEnv()
			catalog, err := resolution.DiscoverPluginCatalog(external.DiscoverOptions{
				PreferExternal: prefer,
				IncludeInvalid: showInvalid,
				PinnedVersions: nil,
			})
			reg := catalog.Registry
			if showInvalid && opts.JSONOutput {
				enc := json.NewEncoder(c.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{"plugins": reg.List(""), "invalid": catalog.Invalid})
			}
			var k manifests.PluginKind
			switch {
			case kind == "":
				// all
			default:
				parsedKind, perr := parsePluginKindFlag(kind)
				if perr != nil {
					return perr
				}
				k = parsedKind
			}
			list := reg.List(k)
			if opts.JSONOutput {
				out := map[string]any{"plugins": list}
				enc := json.NewEncoder(c.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}
			tw := tabwriter.NewWriter(c.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "ID\tKIND\tSOURCE\tVERSION\tDESCRIPTION")
			for _, m := range list {
				desc := m.Description
				if desc == "" {
					desc = "—"
				}
				src := m.Source
				if src == "" {
					src = "builtin"
				}
				ver := m.Version
				if ver == "" {
					ver = "—"
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", m.ID, m.Kind, src, ver, desc)
			}
			if showInvalid && len(catalog.Invalid) > 0 {
				fmt.Fprintln(c.OutOrStdout(), "\nInvalid / skipped external plugins:")
				for _, inv := range catalog.Invalid {
					where := inv.Path
					if where == "" {
						where = inv.ID
					}
					fmt.Fprintf(c.OutOrStdout(), "- %s (%s) %s: %s\n", inv.ID, inv.Kind, where, inv.Reason)
				}
			}
			if err != nil {
				fmt.Fprintf(c.OutOrStdout(), "\nWarning: external discovery failed: %v\n", err)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&kind, "kind", "", "Filter by kind: provider, runtime, simulator")
	cmd.Flags().BoolVar(&showInvalid, "show-invalid", false, "Show invalid/skipped external plugins and reasons")
	cmd.Flags().BoolVar(&preferExternal, "prefer-external", false, "Prefer external plugin manifests when IDs conflict with built-ins")
	return cmd
}

func newExtensionInfoCmd(opts *GlobalOptions) *cobra.Command {
	var version string
	var preferExternal bool
	cmd := &cobra.Command{
		Use:   "info [id]",
		Short: "Show plugin manifest for a given ID",
		RunE: func(c *cobra.Command, args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("usage: runfabric extension info <id>")
			}
			id := args[0]
			reg := manifests.NewPluginRegistry()
			pins := map[string]string{}
			if version != "" {
				pins[id] = version
			}
			catalog, _ := resolution.DiscoverPluginCatalog(external.DiscoverOptions{
				PreferExternal: preferExternal || external.PreferExternalFromEnv(),
				PinnedVersions: pins,
			})
			reg = catalog.Registry
			m := reg.Get(id)
			if m == nil {
				return fmt.Errorf("plugin %q not found", id)
			}
			if opts.JSONOutput {
				enc := json.NewEncoder(c.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(m)
			}
			fmt.Fprintf(c.OutOrStdout(), "id:          %s\n", m.ID)
			fmt.Fprintf(c.OutOrStdout(), "kind:        %s\n", m.Kind)
			fmt.Fprintf(c.OutOrStdout(), "name:        %s\n", m.Name)
			fmt.Fprintf(c.OutOrStdout(), "description: %s\n", m.Description)
			if m.Source != "" {
				fmt.Fprintf(c.OutOrStdout(), "source:      %s\n", m.Source)
			}
			if m.Version != "" {
				fmt.Fprintf(c.OutOrStdout(), "version:     %s\n", m.Version)
			}
			if m.Path != "" {
				fmt.Fprintf(c.OutOrStdout(), "path:        %s\n", m.Path)
			}
			if m.Executable != "" {
				fmt.Fprintf(c.OutOrStdout(), "executable:  %s\n", m.Executable)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&version, "version", "", "Select a specific external plugin version for this ID (best-effort)")
	cmd.Flags().BoolVar(&preferExternal, "prefer-external", false, "Prefer external plugin manifests when IDs conflict with built-ins")
	return cmd
}

func newExtensionSearchCmd(opts *GlobalOptions) *cobra.Command {
	var preferExternal bool
	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Search plugins by id or name (no public marketplace yet)",
		RunE: func(c *cobra.Command, args []string) error {
			query := ""
			if len(args) > 0 {
				query = args[0]
			}
			catalog, _ := resolution.DiscoverPluginCatalog(external.DiscoverOptions{
				PreferExternal: preferExternal || external.PreferExternalFromEnv(),
			})
			list := catalog.Registry.Search(query)
			if opts.JSONOutput {
				out := map[string]any{"plugins": list}
				enc := json.NewEncoder(c.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}
			for _, m := range list {
				if m.Description != "" {
					fmt.Fprintf(c.OutOrStdout(), "%s (%s) — %s\n", m.ID, m.Kind, m.Description)
				} else {
					fmt.Fprintf(c.OutOrStdout(), "%s (%s)\n", m.ID, m.Kind)
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&preferExternal, "prefer-external", false, "Prefer external plugin manifests when IDs conflict with built-ins")
	return cmd
}

func parsePluginKindFlag(raw string) (manifests.PluginKind, error) {
	kind := manifests.NormalizePluginKind(raw)
	if !manifests.IsSupportedPluginKind(kind) {
		return "", fmt.Errorf("--kind must be provider, runtime, or simulator")
	}
	return kind, nil
}
