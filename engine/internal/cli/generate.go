package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/runfabric/runfabric/engine/internal/config"
	"github.com/runfabric/runfabric/engine/internal/configpatch"
	"github.com/runfabric/runfabric/engine/internal/planner"
	"github.com/runfabric/runfabric/engine/internal/scaffold"
	"github.com/spf13/cobra"
)

// generateOpts holds flags for runfabric generate function.
type generateOpts struct {
	Trigger   string
	Route     string
	Schedule  string
	QueueName string
	Provider  string
	Lang      string
	Entry     string
	DryRun    bool
	Force     bool
	NoBackup  bool
}

func newGenerateCmd(opts *GlobalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Scaffold artifacts in an existing project",
		Long:  "Add new functions, resources, addons, or provider overrides without hand-editing runfabric.yml. Use 'runfabric generate function <name>', 'generate resource <name>', 'generate addon <name>', or 'generate provider-override <key>'.",
	}
	cmd.AddCommand(newGenerateFunctionCmd(opts))
	cmd.AddCommand(newGenerateResourceCmd(opts))
	cmd.AddCommand(newGenerateAddonCmd(opts))
	cmd.AddCommand(newGenerateProviderOverrideCmd(opts))
	return cmd
}

func newGenerateFunctionCmd(opts *GlobalOptions) *cobra.Command {
	o := &generateOpts{}

	cmd := &cobra.Command{
		Use:   "function [name]",
		Short: "Generate a new function and patch runfabric.yml",
		Long:  "Scaffolds a handler file and adds the function to runfabric.yml with the given trigger (http, cron, queue). Infers provider and language from config and project; use --trigger, --route, --schedule, --queue-name to customize. Fails if the function name already exists.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var name string
			if len(args) > 0 {
				name = strings.TrimSpace(args[0])
			}
			return runGenerateFunction(opts, o, name)
		},
	}

	cmd.Flags().StringVar(&o.Trigger, "trigger", "http", "Trigger type: http, cron, queue")
	cmd.Flags().StringVar(&o.Route, "route", "", "HTTP route (e.g. GET:/hello)")
	cmd.Flags().StringVar(&o.Schedule, "schedule", "", "Cron schedule (e.g. rate(5 minutes))")
	cmd.Flags().StringVar(&o.QueueName, "queue-name", "", "Queue name for queue trigger")
	cmd.Flags().StringVar(&o.Provider, "provider", "", "Provider override (default: from runfabric.yml)")
	cmd.Flags().StringVar(&o.Lang, "lang", "", "Language: js, ts, python, go (default: infer from config/project)")
	cmd.Flags().StringVar(&o.Entry, "entry", "", "Custom handler path (default: src/<name>.<ext>)")
	cmd.Flags().BoolVar(&o.DryRun, "dry-run", false, "Preview files and config diff without writing")
	cmd.Flags().BoolVar(&o.Force, "force", false, "Overwrite handler file if it exists (never overwrites config)")
	cmd.Flags().BoolVar(&o.NoBackup, "no-backup", false, "Do not create runfabric.yml.bak before patching")

	return cmd
}

func runGenerateFunction(gopts *GlobalOptions, o *generateOpts, name string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	configPath, err := configpatch.ResolveConfigPath(gopts.ConfigPath, cwd, 5)
	if err != nil {
		return err
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	provider := cfg.Provider.Name
	if o.Provider != "" {
		if cfg.ProviderOverrides != nil {
			if override, ok := cfg.ProviderOverrides[o.Provider]; ok {
				provider = override.Name
			} else {
				provider = o.Provider
			}
		} else {
			provider = o.Provider
		}
	}
	if provider == "" {
		provider = "aws-lambda"
	}

	trigger := strings.TrimSpace(strings.ToLower(o.Trigger))
	if trigger == "api" {
		trigger = "http"
	}
	if trigger == "worker" {
		trigger = "queue"
	}
	if trigger != planner.TriggerHTTP && trigger != planner.TriggerCron && trigger != planner.TriggerQueue {
		return fmt.Errorf("--trigger must be http, cron, or queue (got %q)", o.Trigger)
	}

	if !planner.SupportsTrigger(provider, trigger) {
		return fmt.Errorf("provider %q does not support trigger %q", provider, trigger)
	}

	if name == "" {
		return fmt.Errorf("function name is required (e.g. runfabric generate function hello --trigger http)")
	}
	// Sanitize name for use in paths
	if strings.ContainsAny(name, "/\\") {
		return fmt.Errorf("function name must not contain path separators")
	}

	lang := o.Lang
	if lang == "" {
		lang = inferLang(configPath, cfg.Provider.Runtime)
	}
	lang = strings.ToLower(lang)
	if lang == "node" {
		lang = "js"
	}

	handlerResult, ok := scaffold.HandlerContent(lang, trigger)
	if !ok {
		return fmt.Errorf("unsupported language %q", lang)
	}

	projectDir := filepath.Dir(configPath)
	var handlerPath string
	if o.Entry != "" {
		handlerPath = filepath.ToSlash(o.Entry)
	} else {
		handlerPath = "src/" + name + handlerResult.Ext
	}

	route := o.Route
	if trigger == planner.TriggerHTTP && route == "" {
		route = "GET:/" + name
	}

	entry := scaffold.BuildFunctionEntry(handlerPath, trigger, route, o.Schedule, o.QueueName)

	if o.DryRun {
		fragment, collision, err := configpatch.PlanAddFunction(configPath, name, entry)
		if err != nil {
			return err
		}
		if gopts.JSONOutput {
			keys := 0
			if fragment != nil {
				keys = len(fragment)
			}
			fmt.Fprintf(os.Stdout, `{"handler_file":"%s","config_function":"%s","collision":%t,"functions_fragment_keys":%d}`+"\n",
				filepath.Join(projectDir, handlerPath), name, collision, keys)
			return nil
		}
		if collision {
			return fmt.Errorf("function %q already exists in runfabric.yml (dry-run)", name)
		}
		fmt.Fprintf(os.Stdout, "Dry-run: would create\n  %s\nand add function %q to runfabric.yml with trigger %s\n",
			filepath.Join(projectDir, handlerPath), name, trigger)
		if trigger == planner.TriggerHTTP && route != "" {
			fmt.Fprintf(os.Stdout, "  route: %s\n", route)
		}
		return nil
	}

	// Collision check
	if _, exists := cfg.Functions[name]; exists {
		return fmt.Errorf("function %q already exists in runfabric.yml; choose another name or remove it first", name)
	}

	// Write handler file
	absHandler := filepath.Join(projectDir, handlerPath)
	dir := filepath.Dir(absHandler)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create handler dir: %w", err)
	}
	if _, err := os.Stat(absHandler); err == nil && !o.Force {
		return fmt.Errorf("handler file already exists: %s (use --force to overwrite)", handlerPath)
	}
	if err := os.WriteFile(absHandler, []byte(handlerResult.Content), 0o644); err != nil {
		return fmt.Errorf("write handler: %w", err)
	}

	// Patch config
	patchOpts := configpatch.AddFunctionOptions{Backup: !o.NoBackup}
	if err := configpatch.AddFunction(configPath, name, entry, patchOpts); err != nil {
		return err
	}

	if gopts.JSONOutput {
		fmt.Fprintf(os.Stdout, `{"handler_file":"%s","config_updated":true,"function":"%s"}`+"\n", absHandler, name)
		return nil
	}
	fmt.Fprintf(os.Stdout, "Created %s\nAdded function %q to runfabric.yml\nNext: runfabric plan && runfabric deploy\n", handlerPath, name)
	return nil
}

// inferLang infers language from provider runtime and project files next to config.
func inferLang(configPath, runtime string) string {
	dir := filepath.Dir(configPath)
	// From runtime string
	if strings.HasPrefix(runtime, "nodejs") {
		if _, err := os.Stat(filepath.Join(dir, "tsconfig.json")); err == nil {
			return "ts"
		}
		return "js"
	}
	if strings.HasPrefix(runtime, "python") {
		return "python"
	}
	if strings.HasPrefix(runtime, "go") {
		return "go"
	}
	// Detect from project
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
		return "go"
	}
	if _, err := os.Stat(filepath.Join(dir, "requirements.txt")); err == nil {
		return "python"
	}
	if _, err := os.Stat(filepath.Join(dir, "tsconfig.json")); err == nil {
		return "ts"
	}
	if _, err := os.Stat(filepath.Join(dir, "package.json")); err == nil {
		return "js"
	}
	return "js"
}

// generateResourceOpts holds flags for runfabric generate resource.
type generateResourceOpts struct {
	Type          string
	ConnectionEnv string
	DryRun        bool
	NoBackup      bool
}

func newGenerateResourceCmd(opts *GlobalOptions) *cobra.Command {
	o := &generateResourceOpts{}
	cmd := &cobra.Command{
		Use:   "resource [name]",
		Short: "Add a resource entry to runfabric.yml",
		Long:  "Adds resources.<name> (database, cache, or queue) with connection env var. Use --type and --connection-env.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := ""
			if len(args) > 0 {
				name = strings.TrimSpace(args[0])
			}
			return runGenerateResource(opts, o, name)
		},
	}
	cmd.Flags().StringVar(&o.Type, "type", "database", "Resource type: database, cache, queue")
	cmd.Flags().StringVar(&o.ConnectionEnv, "connection-env", "DATABASE_URL", "Env var for connection (e.g. DATABASE_URL, REDIS_URL)")
	cmd.Flags().BoolVar(&o.DryRun, "dry-run", false, "Preview only, do not write")
	cmd.Flags().BoolVar(&o.NoBackup, "no-backup", false, "Do not create runfabric.yml.bak")
	return cmd
}

func runGenerateResource(gopts *GlobalOptions, o *generateResourceOpts, name string) error {
	if name == "" {
		return fmt.Errorf("resource name is required (e.g. runfabric generate resource db --type database)")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	configPath, err := configpatch.ResolveConfigPath(gopts.ConfigPath, cwd, 5)
	if err != nil {
		return err
	}
	typ := strings.ToLower(strings.TrimSpace(o.Type))
	if typ != "database" && typ != "cache" && typ != "queue" {
		return fmt.Errorf("--type must be database, cache, or queue (got %q)", o.Type)
	}
	entry := scaffold.BuildResourceEntry(typ, strings.TrimSpace(o.ConnectionEnv))
	if o.DryRun {
		if gopts.JSONOutput {
			fmt.Fprintf(os.Stdout, `{"resource":"%s","type":"%s","connection_env":"%s","dry_run":true}`+"\n", name, typ, o.ConnectionEnv)
			return nil
		}
		fmt.Fprintf(os.Stdout, "Dry-run: would add resources.%s to runfabric.yml (type=%s, connectionEnv=%s)\n", name, typ, o.ConnectionEnv)
		return nil
	}
	patchOpts := configpatch.AddResourceOptions{Backup: !o.NoBackup}
	if err := configpatch.AddResource(configPath, name, entry, patchOpts); err != nil {
		return err
	}
	if gopts.JSONOutput {
		fmt.Fprintf(os.Stdout, `{"resource":"%s","config_updated":true}`+"\n", name)
		return nil
	}
	fmt.Fprintf(os.Stdout, "Added resource %q to runfabric.yml (type=%s). Next: reference in functions via resources: [%s]\n", name, typ, name)
	return nil
}

// generateAddonOpts holds flags for runfabric generate addon.
type generateAddonOpts struct {
	Version  string
	DryRun   bool
	NoBackup bool
}

func newGenerateAddonCmd(opts *GlobalOptions) *cobra.Command {
	o := &generateAddonOpts{}
	cmd := &cobra.Command{
		Use:   "addon [name]",
		Short: "Add an addon entry to runfabric.yml",
		Long:  "Adds addons.<name> with optional version. Use --version for a specific addon version.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := ""
			if len(args) > 0 {
				name = strings.TrimSpace(args[0])
			}
			return runGenerateAddon(opts, o, name)
		},
	}
	cmd.Flags().StringVar(&o.Version, "version", "", "Addon version (optional)")
	cmd.Flags().BoolVar(&o.DryRun, "dry-run", false, "Preview only")
	cmd.Flags().BoolVar(&o.NoBackup, "no-backup", false, "Do not create runfabric.yml.bak")
	return cmd
}

func runGenerateAddon(gopts *GlobalOptions, o *generateAddonOpts, name string) error {
	if name == "" {
		return fmt.Errorf("addon name is required (e.g. runfabric generate addon sentry)")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	configPath, err := configpatch.ResolveConfigPath(gopts.ConfigPath, cwd, 5)
	if err != nil {
		return err
	}
	entry := scaffold.BuildAddonEntry(strings.TrimSpace(o.Version))
	if o.DryRun {
		if gopts.JSONOutput {
			fmt.Fprintf(os.Stdout, `{"addon":"%s","version":"%s","dry_run":true}`+"\n", name, o.Version)
			return nil
		}
		fmt.Fprintf(os.Stdout, "Dry-run: would add addons.%s to runfabric.yml (version=%s)\n", name, o.Version)
		return nil
	}
	patchOpts := configpatch.AddAddonOptions{Backup: !o.NoBackup}
	if err := configpatch.AddAddon(configPath, name, entry, patchOpts); err != nil {
		return err
	}
	if gopts.JSONOutput {
		fmt.Fprintf(os.Stdout, `{"addon":"%s","config_updated":true}`+"\n", name)
		return nil
	}
	fmt.Fprintf(os.Stdout, "Added addon %q to runfabric.yml. Next: attach to functions via functions.<name>.addons: [%s]\n", name, name)
	return nil
}

// generateProviderOverrideOpts holds flags for runfabric generate provider-override.
type generateProviderOverrideOpts struct {
	Provider string
	Runtime  string
	Region   string
	DryRun   bool
	NoBackup bool
}

func newGenerateProviderOverrideCmd(opts *GlobalOptions) *cobra.Command {
	o := &generateProviderOverrideOpts{}
	cmd := &cobra.Command{
		Use:   "provider-override [key]",
		Short: "Add a provider override to runfabric.yml",
		Long:  "Adds providerOverrides.<key> for multi-cloud (e.g. key=aws, provider=aws-lambda, region=us-east-1). Use with runfabric deploy --provider <key>.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := ""
			if len(args) > 0 {
				key = strings.TrimSpace(args[0])
			}
			return runGenerateProviderOverride(opts, o, key)
		},
	}
	cmd.Flags().StringVar(&o.Provider, "provider", "aws-lambda", "Provider name (e.g. aws-lambda, gcp-functions)")
	cmd.Flags().StringVar(&o.Runtime, "runtime", "nodejs20.x", "Runtime (e.g. nodejs20.x)")
	cmd.Flags().StringVar(&o.Region, "region", "us-east-1", "Region (e.g. us-east-1)")
	cmd.Flags().BoolVar(&o.DryRun, "dry-run", false, "Preview only")
	cmd.Flags().BoolVar(&o.NoBackup, "no-backup", false, "Do not create runfabric.yml.bak")
	return cmd
}

func runGenerateProviderOverride(gopts *GlobalOptions, o *generateProviderOverrideOpts, key string) error {
	if key == "" {
		return fmt.Errorf("provider override key is required (e.g. runfabric generate provider-override aws --provider aws-lambda)")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	configPath, err := configpatch.ResolveConfigPath(gopts.ConfigPath, cwd, 5)
	if err != nil {
		return err
	}
	entry := scaffold.BuildProviderOverrideEntry(
		strings.TrimSpace(o.Provider),
		strings.TrimSpace(o.Runtime),
		strings.TrimSpace(o.Region),
	)
	if o.DryRun {
		if gopts.JSONOutput {
			fmt.Fprintf(os.Stdout, `{"key":"%s","provider":"%s","dry_run":true}`+"\n", key, o.Provider)
			return nil
		}
		fmt.Fprintf(os.Stdout, "Dry-run: would add providerOverrides.%s (provider=%s, runtime=%s, region=%s)\n", key, o.Provider, o.Runtime, o.Region)
		return nil
	}
	patchOpts := configpatch.AddProviderOverrideOptions{Backup: !o.NoBackup}
	if err := configpatch.AddProviderOverride(configPath, key, entry, patchOpts); err != nil {
		return err
	}
	if gopts.JSONOutput {
		fmt.Fprintf(os.Stdout, `{"key":"%s","config_updated":true}`+"\n", key)
		return nil
	}
	fmt.Fprintf(os.Stdout, "Added provider override %q to runfabric.yml. Next: runfabric deploy --provider %s\n", key, key)
	return nil
}
