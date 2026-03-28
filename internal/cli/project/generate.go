package project

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/runfabric/runfabric/internal/cli/common"
	"github.com/runfabric/runfabric/platform/core/model/config"
	"github.com/runfabric/runfabric/platform/core/model/configpatch"
	providerloader "github.com/runfabric/runfabric/platform/extensions/registry/loader/providers"
	scaffold "github.com/runfabric/runfabric/platform/generator/application"
	planner "github.com/runfabric/runfabric/platform/planner/engine"
	"github.com/spf13/cobra"
)

// generateOpts holds flags for runfabric generate function.
type generateOpts struct {
	Trigger       string
	Route         string
	Schedule      string
	QueueName     string
	Provider      string
	Lang          string
	Entry         string
	DryRun        bool
	Force         bool
	NoBackup      bool
	Interactive   bool
	NoInteractive bool
}

func resolveGenerateInteractivity(gopts *common.GlobalOptions, interactive, noInteractive bool) (bool, error) {
	if interactive && noInteractive {
		return false, fmt.Errorf("--interactive and --no-interactive cannot be used together")
	}
	if noInteractive || gopts.NonInteractive {
		return false, nil
	}
	return interactive, nil
}

func promptYesNo(msg string, defaultYes bool) bool {
	def := "y"
	if !defaultYes {
		def = "n"
	}
	for {
		v := strings.ToLower(strings.TrimSpace(promptLine(msg+" [y/n]", def)))
		switch v {
		case "y", "yes":
			return true
		case "n", "no":
			return false
		default:
			fmt.Fprintln(os.Stderr, "Please enter y or n.")
		}
	}
}

func promptGenerateName(kind, current, fallback string) string {
	def := strings.TrimSpace(current)
	if def == "" {
		def = fallback
	}
	for {
		v := strings.TrimSpace(promptLine(kind+" name", def))
		if v == "" {
			fmt.Fprintf(os.Stderr, "%s name is required.\n", kind)
			continue
		}
		if strings.ContainsAny(v, "/\\") {
			fmt.Fprintln(os.Stderr, "Name must not contain path separators.")
			continue
		}
		return v
	}
}

func promptOneOf(label, current string, allowed []string, fallback string) string {
	def := strings.TrimSpace(current)
	if def == "" {
		def = fallback
	}
	allowedSet := map[string]struct{}{}
	for _, a := range allowed {
		allowedSet[a] = struct{}{}
	}
	for {
		v := strings.ToLower(strings.TrimSpace(promptLine(label+" ("+strings.Join(allowed, "|")+")", def)))
		if v == "" {
			v = def
		}
		if _, ok := allowedSet[v]; ok {
			return v
		}
		fmt.Fprintf(os.Stderr, "Invalid value %q. Allowed: %s\n", v, strings.Join(allowed, ", "))
	}
}

func supportedProviders() []string {
	catalog, err := providerloader.NewDefaultProviderCapabilityCatalog()
	if err == nil {
		items, lerr := catalog.ListProviders()
		if lerr == nil && len(items) > 0 {
			providers := make([]string, 0, len(items))
			for _, item := range items {
				providers = append(providers, item.ID)
			}
			sort.Strings(providers)
			return providers
		}
	}

	providers := make([]string, 0, len(planner.ProviderCapabilities))
	for provider := range planner.ProviderCapabilities {
		providers = append(providers, provider)
	}
	sort.Strings(providers)
	return providers
}

func summarizeUnsupportedTriggers(cfg *config.Config, provider string) []string {
	triggers := planner.ExtractTriggers(cfg)
	var unsupported []string
	for _, ft := range triggers {
		for _, spec := range ft.Specs {
			if providerSupportsTrigger(provider, spec.Kind) {
				continue
			}
			unsupported = append(unsupported, fmt.Sprintf("%s:%s", ft.Function, spec.Kind))
		}
	}
	sort.Strings(unsupported)
	return unsupported
}

func providerSupportsTrigger(provider, trigger string) bool {
	catalog, err := providerloader.NewDefaultProviderCapabilityCatalog()
	if err == nil {
		if ok, terr := catalog.SupportsTrigger(provider, trigger); terr == nil {
			return ok
		}
	}
	return planner.SupportsTrigger(provider, trigger)
}

func confirmGeneratePreview(interactive bool, lines ...string) error {
	if !interactive {
		return nil
	}
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Preview:")
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			continue
		}
		fmt.Fprintln(os.Stderr, "  "+l)
	}
	if !promptYesNo("Apply these changes?", true) {
		return fmt.Errorf("operation canceled by user")
	}
	return nil
}

func newGenerateCmd(opts *common.GlobalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Scaffold artifacts in an existing project",
		Long:  "Add new functions, resources, addons, provider overrides, or provider plugin boilerplate without hand-editing runfabric.yml. Use 'runfabric generate function <name>', 'generate resource <name>', 'generate addon <name>', 'generate provider-override <key>', or 'generate plugin <provider-id>'.",
	}
	cmd.AddCommand(newGenerateFunctionCmd(opts))
	cmd.AddCommand(newGenerateWorkerCmd(opts))
	cmd.AddCommand(newGenerateResourceCmd(opts))
	cmd.AddCommand(newGenerateAddonCmd(opts))
	cmd.AddCommand(newGenerateProviderOverrideCmd(opts))
	cmd.AddCommand(newGeneratePluginCmd(opts))
	return cmd
}

func newGenerateWorkerCmd(opts *common.GlobalOptions) *cobra.Command {
	o := &generateOpts{Trigger: planner.TriggerQueue}

	cmd := &cobra.Command{
		Use:   "worker [name]",
		Short: "Generate a queue worker function",
		Long:  "Scaffolds a worker function and patches runfabric.yml with a queue trigger by default. Equivalent to 'runfabric generate function <name> --trigger queue'.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var name string
			if len(args) > 0 {
				name = strings.TrimSpace(args[0])
			}
			if strings.TrimSpace(o.Trigger) == "" {
				o.Trigger = planner.TriggerQueue
			}
			return runGenerateFunction(opts, o, name)
		},
	}

	cmd.Flags().StringVar(&o.Trigger, "trigger", planner.TriggerQueue, "Trigger type (default: queue)")
	cmd.Flags().StringVar(&o.QueueName, "queue-name", "", "Queue name for queue trigger")
	cmd.Flags().StringVar(&o.Provider, "provider", "", "Provider override (default: from runfabric.yml)")
	cmd.Flags().StringVar(&o.Lang, "lang", "", "Language: js, ts, python, go (default: infer from config/project)")
	cmd.Flags().StringVar(&o.Entry, "entry", "", "Custom handler path (default: src/<name>.<ext>)")
	cmd.Flags().BoolVar(&o.DryRun, "dry-run", false, "Preview files and config diff without writing")
	cmd.Flags().BoolVar(&o.Force, "force", false, "Overwrite handler file if it exists (never overwrites config)")
	cmd.Flags().BoolVar(&o.NoBackup, "no-backup", false, "Do not create runfabric.yml.bak before patching")
	cmd.Flags().BoolVar(&o.Interactive, "interactive", false, "Prompt for missing inputs")
	cmd.Flags().BoolVar(&o.NoInteractive, "no-interactive", false, "Disable prompts and require explicit args/flags")

	return cmd
}

func newGenerateFunctionCmd(opts *common.GlobalOptions) *cobra.Command {
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

	cmd.Flags().StringVar(&o.Trigger, "trigger", "", "Trigger type: http, cron, queue")
	cmd.Flags().StringVar(&o.Route, "route", "", "HTTP route (e.g. GET:/hello)")
	cmd.Flags().StringVar(&o.Schedule, "schedule", "", "Cron schedule (e.g. rate(5 minutes))")
	cmd.Flags().StringVar(&o.QueueName, "queue-name", "", "Queue name for queue trigger")
	cmd.Flags().StringVar(&o.Provider, "provider", "", "Provider override (default: from runfabric.yml)")
	cmd.Flags().StringVar(&o.Lang, "lang", "", "Language: js, ts, python, go (default: infer from config/project)")
	cmd.Flags().StringVar(&o.Entry, "entry", "", "Custom handler path (default: src/<name>.<ext>)")
	cmd.Flags().BoolVar(&o.DryRun, "dry-run", false, "Preview files and config diff without writing")
	cmd.Flags().BoolVar(&o.Force, "force", false, "Overwrite handler file if it exists (never overwrites config)")
	cmd.Flags().BoolVar(&o.NoBackup, "no-backup", false, "Do not create runfabric.yml.bak before patching")
	cmd.Flags().BoolVar(&o.Interactive, "interactive", false, "Prompt for missing inputs")
	cmd.Flags().BoolVar(&o.NoInteractive, "no-interactive", false, "Disable prompts and require explicit args/flags")

	return cmd
}

func runGenerateFunction(gopts *common.GlobalOptions, o *generateOpts, name string) error {
	interactive, err := resolveGenerateInteractivity(gopts, o.Interactive, o.NoInteractive)
	if err != nil {
		return err
	}
	if interactive {
		name = promptGenerateName("Function", name, "hello")
	}

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

	lang := strings.ToLower(strings.TrimSpace(o.Lang))
	if lang == "node" {
		lang = "js"
	}
	inferredLang := inferLang(configPath, cfg.Provider.Runtime)
	if interactive {
		lang = promptOneOf("Language", lang, []string{"js", "ts", "python", "go"}, inferredLang)
	} else if lang == "" {
		lang = strings.ToLower(inferredLang)
		if lang == "node" {
			lang = "js"
		}
	}

	trigger := strings.TrimSpace(strings.ToLower(o.Trigger))
	if trigger == "api" {
		trigger = "http"
	}
	if trigger == "worker" {
		trigger = "queue"
	}
	if interactive {
		trigger = promptOneOf("Trigger", trigger, []string{"http", "cron", "queue"}, "http")
	}
	if trigger == "" {
		trigger = planner.TriggerHTTP
	}
	if trigger != planner.TriggerHTTP && trigger != planner.TriggerCron && trigger != planner.TriggerQueue {
		return fmt.Errorf("--trigger must be http, cron, or queue (got %q)", o.Trigger)
	}
	if !providerSupportsTrigger(provider, trigger) {
		if interactive {
			for {
				fmt.Fprintf(os.Stderr, "Provider %q does not support trigger %q.\n", provider, trigger)
				trigger = promptOneOf("Trigger", "", []string{"http", "cron", "queue"}, "http")
				if providerSupportsTrigger(provider, trigger) {
					break
				}
			}
		} else {
			return fmt.Errorf("provider %q does not support trigger %q", provider, trigger)
		}
	}

	if name == "" {
		return fmt.Errorf("function name is required (e.g. runfabric generate function hello --trigger http, or use --interactive)")
	}
	// Sanitize name for use in paths
	if strings.ContainsAny(name, "/\\") {
		return fmt.Errorf("function name must not contain path separators")
	}

	handlerResult, ok := scaffold.HandlerContent(lang, trigger)
	if !ok {
		if interactive {
			for {
				fmt.Fprintf(os.Stderr, "Unsupported language %q for trigger %q.\n", lang, trigger)
				lang = promptOneOf("Language", "", []string{"js", "ts", "python", "go"}, inferredLang)
				handlerResult, ok = scaffold.HandlerContent(lang, trigger)
				if ok {
					break
				}
			}
		} else {
			return fmt.Errorf("unsupported language %q", lang)
		}
	}

	projectDir := filepath.Dir(configPath)
	var handlerPath string
	if interactive {
		entryDefault := "src/" + name + handlerResult.Ext
		handlerPath = filepath.ToSlash(strings.TrimSpace(promptLine("Entry path", entryDefault)))
	} else if o.Entry != "" {
		handlerPath = filepath.ToSlash(o.Entry)
	} else {
		handlerPath = "src/" + name + handlerResult.Ext
	}

	route := strings.TrimSpace(o.Route)
	if interactive {
		switch trigger {
		case planner.TriggerHTTP:
			def := route
			if def == "" {
				def = "GET:/" + name
			}
			route = strings.TrimSpace(promptLine("HTTP route (METHOD:/path)", def))
		case planner.TriggerCron:
			o.Schedule = strings.TrimSpace(promptLine("Cron schedule", strings.TrimSpace(o.Schedule)))
		case planner.TriggerQueue:
			o.QueueName = strings.TrimSpace(promptLine("Queue name", strings.TrimSpace(o.QueueName)))
		}
	}
	if trigger == planner.TriggerHTTP && route == "" {
		route = "GET:/" + name
	}
	if trigger == planner.TriggerCron && strings.TrimSpace(o.Schedule) == "" {
		return fmt.Errorf("--schedule is required for cron trigger")
	}
	if trigger == planner.TriggerQueue && strings.TrimSpace(o.QueueName) == "" {
		return fmt.Errorf("--queue-name is required for queue trigger")
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
		if !interactive {
			return fmt.Errorf("function %q already exists in runfabric.yml; choose another name or remove it first", name)
		}
		for {
			name = promptGenerateName("Function", "", name+"2")
			if _, exists := cfg.Functions[name]; !exists {
				break
			}
			fmt.Fprintf(os.Stderr, "Function %q already exists.\n", name)
		}
		handlerPath = "src/" + name + handlerResult.Ext
		if trigger == planner.TriggerHTTP && strings.TrimSpace(o.Route) == "" {
			route = "GET:/" + name
		}
		entry = scaffold.BuildFunctionEntry(handlerPath, trigger, route, o.Schedule, o.QueueName)
	}

	absHandler := filepath.Join(projectDir, handlerPath)
	if err := confirmGeneratePreview(interactive,
		"create file: "+absHandler,
		"patch runfabric.yml: add function \""+name+"\"",
		"trigger: "+trigger,
		func() string {
			if trigger == planner.TriggerHTTP {
				return "route: " + route
			}
			if trigger == planner.TriggerCron {
				return "schedule: " + strings.TrimSpace(o.Schedule)
			}
			if trigger == planner.TriggerQueue {
				return "queue: " + strings.TrimSpace(o.QueueName)
			}
			return ""
		}(),
	); err != nil {
		return err
	}

	// Write handler file
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
	Interactive   bool
	NoInteractive bool
}

func newGenerateResourceCmd(opts *common.GlobalOptions) *cobra.Command {
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
	cmd.Flags().BoolVar(&o.Interactive, "interactive", false, "Prompt for missing inputs")
	cmd.Flags().BoolVar(&o.NoInteractive, "no-interactive", false, "Disable prompts and require explicit args/flags")
	return cmd
}

func runGenerateResource(gopts *common.GlobalOptions, o *generateResourceOpts, name string) error {
	interactive, err := resolveGenerateInteractivity(gopts, o.Interactive, o.NoInteractive)
	if err != nil {
		return err
	}
	if interactive {
		name = promptGenerateName("Resource", name, "db")
		o.Type = promptOneOf("Resource type", strings.ToLower(strings.TrimSpace(o.Type)), []string{"database", "cache", "queue"}, "database")
		o.ConnectionEnv = strings.TrimSpace(promptLine("Connection env var", strings.TrimSpace(o.ConnectionEnv)))
	}

	if name == "" {
		return fmt.Errorf("resource name is required (e.g. runfabric generate resource db --type database, or use --interactive)")
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
	if err := confirmGeneratePreview(interactive,
		"patch runfabric.yml: add resource \""+name+"\"",
		"type: "+typ,
		"connectionEnv: "+strings.TrimSpace(o.ConnectionEnv),
	); err != nil {
		return err
	}
	patchOpts := configpatch.AddResourceOptions{Backup: !o.NoBackup}
	if err := configpatch.AddResource(configPath, name, entry, patchOpts); err != nil {
		if interactive && strings.Contains(err.Error(), "already exists") {
			for {
				name = promptGenerateName("Resource", "", name+"2")
				if err2 := configpatch.AddResource(configPath, name, entry, patchOpts); err2 == nil {
					if gopts.JSONOutput {
						fmt.Fprintf(os.Stdout, `{"resource":"%s","config_updated":true}`+"\n", name)
						return nil
					}
					fmt.Fprintf(os.Stdout, "Added resource %q to runfabric.yml (type=%s). Next: reference in functions via resources: [%s]\n", name, typ, name)
					return nil
				}
			}
		}
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
	Version       string
	DryRun        bool
	NoBackup      bool
	Interactive   bool
	NoInteractive bool
}

func newGenerateAddonCmd(opts *common.GlobalOptions) *cobra.Command {
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
	cmd.Flags().BoolVar(&o.Interactive, "interactive", false, "Prompt for missing inputs")
	cmd.Flags().BoolVar(&o.NoInteractive, "no-interactive", false, "Disable prompts and require explicit args/flags")
	return cmd
}

func runGenerateAddon(gopts *common.GlobalOptions, o *generateAddonOpts, name string) error {
	interactive, err := resolveGenerateInteractivity(gopts, o.Interactive, o.NoInteractive)
	if err != nil {
		return err
	}
	if interactive {
		name = promptGenerateName("Addon", name, "sentry")
		o.Version = strings.TrimSpace(promptLine("Addon version (optional)", strings.TrimSpace(o.Version)))
	}

	if name == "" {
		return fmt.Errorf("addon name is required (e.g. runfabric generate addon sentry, or use --interactive)")
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
	if err := confirmGeneratePreview(interactive,
		"patch runfabric.yml: add addon \""+name+"\"",
		"version: "+strings.TrimSpace(o.Version),
	); err != nil {
		return err
	}
	patchOpts := configpatch.AddAddonOptions{Backup: !o.NoBackup}
	if err := configpatch.AddAddon(configPath, name, entry, patchOpts); err != nil {
		if interactive && strings.Contains(err.Error(), "already exists") {
			for {
				name = promptGenerateName("Addon", "", name+"2")
				if err2 := configpatch.AddAddon(configPath, name, entry, patchOpts); err2 == nil {
					if gopts.JSONOutput {
						fmt.Fprintf(os.Stdout, `{"addon":"%s","config_updated":true}`+"\n", name)
						return nil
					}
					fmt.Fprintf(os.Stdout, "Added addon %q to runfabric.yml. Next: attach to functions via functions.<name>.addons: [%s]\n", name, name)
					return nil
				}
			}
		}
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
	Provider      string
	Runtime       string
	Region        string
	DryRun        bool
	NoBackup      bool
	Interactive   bool
	NoInteractive bool
}

func newGenerateProviderOverrideCmd(opts *common.GlobalOptions) *cobra.Command {
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
	cmd.Flags().BoolVar(&o.Interactive, "interactive", false, "Prompt for missing inputs")
	cmd.Flags().BoolVar(&o.NoInteractive, "no-interactive", false, "Disable prompts and require explicit args/flags")
	return cmd
}

func runGenerateProviderOverride(gopts *common.GlobalOptions, o *generateProviderOverrideOpts, key string) error {
	interactive, err := resolveGenerateInteractivity(gopts, o.Interactive, o.NoInteractive)
	if err != nil {
		return err
	}

	if key == "" {
		if !interactive {
			return fmt.Errorf("provider override key is required (e.g. runfabric generate provider-override aws --provider aws-lambda, or use --interactive)")
		}
	}
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
	if interactive {
		key = promptGenerateName("Provider override", key, "aws")
		providers := supportedProviders()
		fallbackProvider := strings.TrimSpace(o.Provider)
		if fallbackProvider == "" {
			fallbackProvider = cfg.Provider.Name
		}
		o.Provider = promptOneOf("Provider name", strings.TrimSpace(o.Provider), providers, fallbackProvider)
		for {
			unsupported := summarizeUnsupportedTriggers(cfg, strings.TrimSpace(o.Provider))
			if len(unsupported) == 0 {
				break
			}
			fmt.Fprintf(os.Stderr, "Provider %q does not support existing triggers in this project: %s\n", o.Provider, strings.Join(unsupported, ", "))
			o.Provider = promptOneOf("Provider name", "", providers, cfg.Provider.Name)
		}
		o.Runtime = strings.TrimSpace(promptLine("Runtime", strings.TrimSpace(o.Runtime)))
		o.Region = strings.TrimSpace(promptLine("Region", strings.TrimSpace(o.Region)))
	}
	if unsupported := summarizeUnsupportedTriggers(cfg, strings.TrimSpace(o.Provider)); len(unsupported) > 0 {
		return fmt.Errorf("provider %q does not support existing project triggers: %s (see Trigger Capability Matrix)", o.Provider, strings.Join(unsupported, ", "))
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
	if err := confirmGeneratePreview(interactive,
		"patch runfabric.yml: add providerOverrides."+key,
		"provider: "+strings.TrimSpace(o.Provider),
		"runtime: "+strings.TrimSpace(o.Runtime),
		"region: "+strings.TrimSpace(o.Region),
	); err != nil {
		return err
	}
	patchOpts := configpatch.AddProviderOverrideOptions{Backup: !o.NoBackup}
	if err := configpatch.AddProviderOverride(configPath, key, entry, patchOpts); err != nil {
		if interactive && strings.Contains(err.Error(), "already exists") {
			for {
				key = promptGenerateName("Provider override", "", key+"2")
				if err2 := configpatch.AddProviderOverride(configPath, key, entry, patchOpts); err2 == nil {
					if gopts.JSONOutput {
						fmt.Fprintf(os.Stdout, `{"key":"%s","config_updated":true}`+"\n", key)
						return nil
					}
					fmt.Fprintf(os.Stdout, "Added provider override %q to runfabric.yml. Next: runfabric deploy --provider %s\n", key, key)
					return nil
				}
			}
		}
		return err
	}
	if gopts.JSONOutput {
		fmt.Fprintf(os.Stdout, `{"key":"%s","config_updated":true}`+"\n", key)
		return nil
	}
	fmt.Fprintf(os.Stdout, "Added provider override %q to runfabric.yml. Next: runfabric deploy --provider %s\n", key, key)
	return nil
}
