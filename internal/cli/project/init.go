package project

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/runfabric/runfabric/internal/cli/common"
	"github.com/runfabric/runfabric/platform/extensions/application/external"
	manifests "github.com/runfabric/runfabric/platform/extensions/manifest"
	"github.com/runfabric/runfabric/platform/extensions/providerpolicy"
	providerloader "github.com/runfabric/runfabric/platform/extensions/registry/loader/providers"
	"github.com/runfabric/runfabric/platform/extensions/registry/resolution"
	scaffold "github.com/runfabric/runfabric/platform/generator/application"
	planner "github.com/runfabric/runfabric/platform/planner/engine"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const (
	asciiESC  = 27 // Escape
	asciiETX  = 3  // Ctrl+C
	maxESCSeq = 16 // max bytes to consume for an escape sequence
)

// initOpts holds init-specific flags (aligned with docs/COMMAND_REFERENCE.md).
type initOpts struct {
	Dir             string
	Template        string
	Provider        string
	SecretManager   string
	StateBackend    string
	Lang            string
	Service         string
	PM              string
	SkipInstall     bool
	CallLocal       bool
	NoInteractive   bool
	WithBuildScript bool   // add build script (and for TS: tsconfig + tsc)
	WithCI          string // e.g. "github-actions" to add .github/workflows/deploy.yml
}

var (
	triggerLabels = map[string]string{
		planner.TriggerHTTP:        "HTTP API",
		planner.TriggerCron:        "Scheduled (cron)",
		planner.TriggerQueue:       "Queue (SQS, etc.)",
		planner.TriggerStorage:     "Storage (S3, etc.)",
		planner.TriggerEventBridge: "EventBridge",
		planner.TriggerPubSub:      "Pub/Sub",
	}
	stateBackends = []string{"local", "s3", "gcs", "azblob", "postgres"}
	langs         = []string{"js", "ts", "python", "go"}
	pkgManagers   = []string{"npm", "pnpm", "yarn", "bun"}
)

func newInitCmd(opts *common.GlobalOptions) *cobra.Command {
	initOpts := &initOpts{}

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Scaffold a new runfabric project",
		Long:  "Creates a new project with runfabric.yml and handler scaffolding. Use interactive mode (default) to select provider, trigger, language, and state backend, or pass flags for non-interactive.",
		RunE: func(c *cobra.Command, args []string) error {
			initOpts.NoInteractive = initOpts.NoInteractive || opts.NonInteractive
			if opts.JSONOutput {
				// Headless mode for API callers (the PaaS): generate the project files
				// as JSON instead of writing to disk or prompting.
				return runInitJSON(initOpts)
			}
			return runInit(initOpts)
		},
	}

	cmd.Flags().StringVar(&initOpts.Dir, "dir", "", "Target directory (default: create folder named after service)")
	cmd.Flags().StringVar(&initOpts.Template, "template", "", "Template/trigger: http|cron|queue|storage|eventbridge|pubsub")
	cmd.Flags().StringVar(&initOpts.Provider, "provider", "", "Provider (e.g. aws-lambda, gcp-functions)")
	cmd.Flags().StringVar(&initOpts.SecretManager, "secret-manager", "", "Secret manager plugin ID (dynamic). Use 'none' or empty for default env/secret:// resolution")
	cmd.Flags().StringVar(&initOpts.StateBackend, "state-backend", "", "State backend: local|s3|gcs|azblob|postgres (default: prompt in interactive mode)")
	cmd.Flags().StringVar(&initOpts.Lang, "lang", "", "Language: js|ts|python|go (default: prompt in interactive mode)")
	cmd.Flags().StringVar(&initOpts.Service, "service", "", "Service name (default: from --dir)")
	cmd.Flags().StringVar(&initOpts.PM, "pm", "npm", "Package manager: npm|pnpm|yarn|bun")
	cmd.Flags().BoolVar(&initOpts.SkipInstall, "skip-install", false, "Skip installing dependencies after scaffold")
	cmd.Flags().BoolVar(&initOpts.CallLocal, "call-local", false, "Add a script to run call-local after scaffold")
	cmd.Flags().BoolVar(&initOpts.NoInteractive, "no-interactive", false, "Disable interactive prompts; use flags only")
	cmd.Flags().BoolVar(&initOpts.WithBuildScript, "with-build", true, "Add package.json and (for TypeScript) tsconfig.json with build script (tsc)")
	cmd.Flags().StringVar(&initOpts.WithCI, "with-ci", "", "Add CI workflow: github-actions (doctor → plan → deploy on push)")

	return cmd
}

func runInit(o *initOpts) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	if isNodeLang(o.Lang) && !isValidPackageManager(o.PM) {
		return fmt.Errorf("unsupported --pm %q; use npm, pnpm, yarn, or bun", o.PM)
	}
	var dir string
	if o.Dir != "" {
		dir, err = filepath.Abs(o.Dir)
		if err != nil {
			return err
		}
	}

	// Resolve provider, trigger, lang, state via interactive or flags
	if !o.NoInteractive {
		printInitIntro()
		if o.Provider == "" {
			o.Provider = promptProvider()
		}
		if o.Provider != "" && o.Template == "" {
			o.Template = promptTrigger(o.Provider)
		}
		if o.Lang == "" {
			o.Lang = promptLang()
		}
		if isNodeLang(o.Lang) {
			if o.PM == "" || !isValidPackageManager(o.PM) {
				o.PM = pkgManagers[promptSelect("Select package manager", pkgManagers, 0)]
			}
			if !o.CallLocal {
				o.CallLocal = promptYesNoInit("Add call:local script?", true)
			}
			if !o.SkipInstall {
				o.SkipInstall = !promptYesNoInit("Install dependencies now?", true)
			}
		}
		// When lang is ts, prompt for build script (tsconfig + tsc)
		if o.Lang == "ts" && !o.NoInteractive {
			idx := promptSelect("Add TypeScript build script (tsc)?", []string{"No", "Yes"}, 1)
			o.WithBuildScript = idx == 1
		}
		if o.StateBackend == "" {
			o.StateBackend = promptState()
		}
		if strings.TrimSpace(o.SecretManager) == "" {
			o.SecretManager = promptSecretManager()
		}
		if o.Service == "" {
			defaultService := "my-service"
			if dir != "" {
				defaultService = filepath.Base(dir)
				if defaultService == "" || defaultService == "." || defaultService == "engine" {
					defaultService = "my-service"
				}
			}
			o.Service = promptLine("Service name", defaultService)
		}
	}

	// Service defaults to the target directory name when not given; the remaining
	// defaults/validation and all file generation are shared with the Scaffold API
	// (also used by `init --json` and the daemon POST /scaffold route).
	if o.Service == "" && dir != "" {
		o.Service = filepath.Base(dir)
	}
	res, err := scaffoldProject(o)
	if err != nil {
		return err
	}

	// If --dir was not passed, create a folder named after the (now-defaulted) service.
	if o.Dir == "" {
		dir = filepath.Join(cwd, o.Service)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create dir %s: %w", dir, err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "src"), 0o755); err != nil {
		return fmt.Errorf("create src dir: %w", err)
	}

	// Write the generated files (order + names match the Scaffold result).
	for _, f := range res.Files {
		full := filepath.Join(dir, filepath.FromSlash(f.Path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return fmt.Errorf("create dir for %s: %w", f.Path, err)
		}
		if err := os.WriteFile(full, []byte(f.Content), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", f.Path, err)
		}
		common.InitWrote(f.Path)
	}

	installErr := error(nil)
	if isNodeLang(o.Lang) && !o.SkipInstall {
		fmt.Fprintf(os.Stderr, "\nInstalling dependencies with %s...\n", o.PM)
		if err := installNodeDependencies(dir, o.PM); err != nil {
			installErr = err
			fmt.Fprintf(os.Stderr, "Warning: dependency install failed: %v\n", err)
		}
	}
	if o.Lang == "ts" && o.WithBuildScript && installErr == nil && !o.SkipInstall {
		fmt.Fprintf(os.Stderr, "\nBuilding TypeScript scaffold...\n")
		if err := runNodeScript(dir, o.PM, "build"); err != nil {
			installErr = err
			fmt.Fprintf(os.Stderr, "Warning: initial build failed: %v\n", err)
		}
	}

	// Show paths relative to cwd for "Project ready in" and "Next:"
	projectDirLabel := filepath.Base(dir)
	if cwd, err := os.Getwd(); err == nil {
		if rel, err := filepath.Rel(cwd, dir); err == nil {
			if rel == "." || rel == "" {
				projectDirLabel = "."
			} else {
				projectDirLabel = filepath.ToSlash(rel)
			}
		}
	}
	common.InitReady(projectDirLabel, o.Provider, o.Template, o.Lang, o.StateBackend)
	cdPrefix := ""
	if projectDirLabel != "." {
		cdPrefix = "cd " + projectDirLabel + " && "
	}
	if isNodeLang(o.Lang) && (o.SkipInstall || installErr != nil) {
		common.InitNext(cdPrefix + packageInstallCommand(o.PM))
	}
	if o.Lang == "ts" && o.WithBuildScript {
		common.InitNext(cdPrefix + packageRunCommand(o.PM, "build"))
	}
	common.InitNext(cdPrefix + "runfabric doctor --config runfabric.yml --stage dev")
	if isNodeLang(o.Lang) && o.CallLocal {
		common.InitNext(cdPrefix + packageRunCommand(o.PM, "call:local"))
	}
	return nil
}

func printInitIntro() {
	if !term.IsTerminal(int(os.Stderr.Fd())) {
		return
	}
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "RunFabric project setup")
	fmt.Fprintln(os.Stderr, "Choose provider, trigger, language, and state backend to scaffold your service.")
}

func promptProvider() string {
	list, err := listProviderDescriptors()
	if err == nil && len(list) > 0 {
		providerIDs := make([]string, 0, len(list))
		display := make([]string, 0, len(list))
		for _, item := range list {
			providerIDs = append(providerIDs, item.ID)
			display = append(display, item.ID)
		}
		idx := promptSelect("Select provider", display, 0)
		if idx < 0 {
			return providerIDs[0]
		}
		return providerIDs[idx]
	}

	providers := make([]string, 0, len(planner.ProviderCapabilities))
	for p := range planner.ProviderCapabilities {
		providers = append(providers, p)
	}
	sort.Strings(providers)
	idx := promptSelect("Select provider", providers, 0)
	if idx < 0 {
		return "aws-lambda"
	}
	return providers[idx]
}

func promptTrigger(provider string) string {
	triggers := initProviderSupportedTriggers(provider)
	if len(triggers) == 0 {
		return planner.TriggerHTTP
	}
	sort.Slice(triggers, func(i, j int) bool { return triggers[i] < triggers[j] })
	options := make([]string, len(triggers))
	for i, t := range triggers {
		if label, ok := triggerLabels[t]; ok {
			options[i] = fmt.Sprintf("%s (%s)", t, label)
		} else {
			options[i] = t
		}
	}
	idx := promptSelect("Select trigger", options, 0)
	if idx < 0 {
		return planner.TriggerHTTP
	}
	return triggers[idx]
}

func listProviderDescriptors() ([]providerloader.ProviderDescriptor, error) {
	catalog, err := providerloader.NewDefaultProviderCapabilityCatalog()
	if err != nil {
		return nil, err
	}
	return catalog.ListProviders()
}

func initProviderSupportedTriggers(provider string) []string {
	catalog, err := providerloader.NewDefaultProviderCapabilityCatalog()
	if err == nil {
		if triggers, terr := catalog.SupportedTriggers(provider); terr == nil && len(triggers) > 0 {
			return triggers
		}
	}
	return planner.SupportedTriggers(provider)
}

func initProviderSupportsTrigger(provider, trigger string) bool {
	catalog, err := providerloader.NewDefaultProviderCapabilityCatalog()
	if err == nil {
		if ok, terr := catalog.SupportsTrigger(provider, trigger); terr == nil {
			return ok
		}
	}
	return planner.SupportsTrigger(provider, trigger)
}

func promptLang() string {
	idx := promptSelect("Select language", langs, 1) // default ts
	if idx < 0 {
		return "ts"
	}
	return langs[idx]
}

func promptState() string {
	idx := promptSelect("Select state backend", stateBackends, 0)
	if idx < 0 {
		return "local"
	}
	return stateBackends[idx]
}

func promptSecretManager() string {
	options := secretManagerPromptOptions()
	idx := promptSelect("Select secret manager plugin", options, 0)
	if idx <= 0 {
		return ""
	}
	picked := strings.TrimSpace(options[idx])
	if strings.EqualFold(picked, "none") {
		return ""
	}
	return picked
}

func secretManagerPromptOptions() []string {
	options := []string{"none"}
	ids, err := listSecretManagerPluginIDs()
	if err != nil {
		return options
	}
	return append(options, ids...)
}

func listSecretManagerPluginIDs() ([]string, error) {
	catalog, err := resolution.DiscoverPluginCatalog(external.DiscoverOptions{})
	if err != nil {
		return nil, err
	}
	items := catalog.Registry.List(manifests.KindSecretManager)
	ids := make([]string, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		if item == nil {
			continue
		}
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		key := strings.ToLower(id)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids, nil
}

func normalizeSecretManagerPlugin(raw string) (string, error) {
	v := strings.TrimSpace(raw)
	switch v {
	case "":
		return "", nil
	}
	if strings.EqualFold(v, "none") {
		return "", nil
	}
	ids, err := listSecretManagerPluginIDs()
	if err != nil || len(ids) == 0 {
		// Fallback when discovery is unavailable: accept explicit plugin IDs.
		return v, nil
	}
	for _, id := range ids {
		if strings.EqualFold(id, v) {
			return id, nil
		}
	}
	return "", fmt.Errorf(
		"unsupported --secret-manager %q; available: none, %s",
		raw,
		strings.Join(ids, ", "),
	)
}

func promptYesNoInit(msg string, defaultYes bool) bool {
	defaultIdx := 0
	if !defaultYes {
		defaultIdx = 1
	}
	idx := promptSelect(msg, []string{"Yes", "No"}, defaultIdx)
	return idx == 0
}

func promptSelect(msg string, options []string, defaultIdx int) int {
	if len(options) == 0 {
		return -1
	}
	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		return promptSelectArrow(msg, options, defaultIdx, fd)
	}
	// Non-TTY fallback: number prompt
	fmt.Println()
	fmt.Println(msg + ":")
	for i, opt := range options {
		mark := " "
		if i == defaultIdx {
			mark = ">"
		}
		fmt.Printf("  %s [%d] %s\n", mark, i+1, opt)
	}
	fmt.Printf("  Enter number (1-%d, default %d): ", len(options), defaultIdx+1)
	line := readLine()
	line = strings.TrimSpace(line)
	if line == "" {
		return defaultIdx
	}
	var n int
	if _, err := fmt.Sscanf(line, "%d", &n); err != nil {
		return defaultIdx
	}
	if n < 1 || n > len(options) {
		return defaultIdx
	}
	return n - 1
}

// promptSelectArrow shows a list and lets the user move with ↑/↓ and select with Enter.
func promptSelectArrow(msg string, options []string, defaultIdx int, fd int) int {
	cur := defaultIdx
	if cur >= len(options) {
		cur = len(options) - 1
	}
	if cur < 0 {
		cur = 0
	}

	nOpt := len(options)
	// Move cursor up nOpt lines to first option line, redraw options + hint, then flush so terminal sees the escape
	render := func() {
		fmt.Fprint(os.Stderr, "\033[", nOpt, "A")
		for i, opt := range options {
			fmt.Fprint(os.Stderr, "\r\033[2K")
			if i == cur {
				fmt.Fprintln(os.Stderr, "  \033[1m> "+opt+"\033[0m")
			} else {
				fmt.Fprintln(os.Stderr, "    "+opt)
			}
		}
		fmt.Fprint(os.Stderr, "\033[2K\r  Use ↑/↓ and Enter to select")
		os.Stderr.Sync()
	}

	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, msg+":")
	for i, opt := range options {
		if i == cur {
			fmt.Fprintln(os.Stderr, "  \033[1m> "+opt+"\033[0m")
		} else {
			fmt.Fprintln(os.Stderr, "    "+opt)
		}
	}
	fmt.Fprint(os.Stderr, "  Use ↑/↓ and Enter to select")
	os.Stderr.Sync()

	oldState, err := term.MakeRaw(fd)
	if err != nil {
		fmt.Fprintln(os.Stderr)
		return defaultIdx
	}
	defer term.Restore(fd, oldState)

	buf := make([]byte, 8)
	for {
		nn, readErr := os.Stdin.Read(buf[:1])
		if readErr != nil || nn < 1 {
			break
		}
		b := buf[0]
		if b == '\r' || b == '\n' {
			fmt.Fprintln(os.Stderr)
			return cur
		}
		if b == asciiESC {
			// Consume CSI sequence until terminating letter or max length
			consumed := 0
			for consumed < maxESCSeq {
				nr, _ := os.Stdin.Read(buf[:1])
				if nr < 1 {
					break
				}
				consumed++
				c := buf[0]
				if c == 'A' {
					cur--
					if cur < 0 {
						cur = 0
					}
					render()
					break
				}
				if c == 'B' {
					cur++
					if cur >= len(options) {
						cur = len(options) - 1
					}
					render()
					break
				}
				if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
					break
				}
			}
			continue
		}
		if b == asciiETX {
			term.Restore(fd, oldState)
			os.Exit(130)
		}
	}
	fmt.Fprintln(os.Stderr)
	return cur
}

func promptLine(msg, defaultVal string) string {
	fmt.Fprintln(os.Stderr)
	fmt.Fprint(os.Stderr, msg, " [", defaultVal, "]: ")
	os.Stderr.Sync()
	line := readLine()
	line = strings.TrimSpace(line)
	if line == "" {
		return defaultVal
	}
	return line
}

func readLine() string {
	sc := bufio.NewScanner(os.Stdin)
	if sc.Scan() {
		return sc.Text()
	}
	return ""
}

// yamlQuoted returns a YAML-safe scalar: either the raw value if it is safe
// (alphanumeric, hyphen, underscore, dot only) or a double-quoted escaped string
// to prevent injection of newlines/colons into generated runfabric.yml.
func yamlQuoted(s string) string {
	needQuote := false
	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsNumber(r) && r != '-' && r != '_' && r != '.' {
			needQuote = true
			break
		}
	}
	if !needQuote && s != "" {
		return s
	}
	var b strings.Builder
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		default:
			b.WriteByte(s[i])
		}
	}
	b.WriteByte('"')
	return b.String()
}

// resolveRuntimeEntryExt computes the deploy runtime id, the functions[].entry
// reference, and the handler file extension for the target language/provider.
// Shared by generateRunfabricYAML and the Scaffold API so all three agree.
func resolveRuntimeEntryExt(o *initOpts) (runtime, entry, ext string) {
	// Provider-declared scaffolding deltas (entry override, runtime).
	sc := providerpolicy.ProviderScaffold(o.Provider)
	runtime = "nodejs20.x"
	if o.Lang == "python" {
		runtime = "python3.11"
	}
	if o.Lang == "go" {
		runtime = "go1.x"
	}
	if r, ok := sc.RuntimeByLang[o.Lang]; ok && r != "" {
		runtime = r
	}
	entry = "src/handler.handler"
	ext = ".js"
	if o.Lang == "ts" {
		// TypeScript build outputs to dist/handler.js; deploy uses compiled output
		entry = "dist/handler.handler"
		ext = ".ts"
	}
	if o.Lang == "python" {
		entry = "handler.handler"
		ext = ".py"
	}
	if o.Lang == "go" {
		entry = "handler"
		ext = ".go"
	}
	// Provider entry override (e.g. Cloudflare Workers deploy a worker.js script,
	// not a Lambda-style handler) — applies to plain-JS scaffolds only.
	if jsScaffold(o) && sc.Entry != "" {
		entry = sc.Entry
	}
	return runtime, entry, ext
}

func generateRunfabricYAML(o *initOpts) string {
	// Provider-declared scaffolding deltas (comment, entry override, runtime).
	sc := providerpolicy.ProviderScaffold(o.Provider)
	runtime, handler, ext := resolveRuntimeEntryExt(o)

	var b strings.Builder
	b.WriteString("# RunFabric config — generated by runfabric init\n")
	// Provider-specific comment, declared by the provider (init.go stays generic).
	comment := ""
	if sc.Comment != "" {
		comment = "# " + sc.Comment
	}
	b.WriteString(comment + "\n")
	b.WriteString("service: " + yamlQuoted(o.Service) + "\n\n")
	b.WriteString("provider:\n")
	b.WriteString("  name: " + o.Provider + "\n")
	b.WriteString("  runtime: " + runtime + "\n")
	// Region line derived from the provider's own credential surface (the *_REGION
	// var) rather than defaulting every provider to AWS_REGION; providers without a
	// region credential (azure/cloudflare/vercel/kubernetes/…) omit it.
	if line := providerRegionLine(o.Provider); line != "" {
		b.WriteString("  region: " + line + "\n")
	}
	b.WriteString("\n")

	if o.StateBackend != "local" {
		b.WriteString("backend:\n")
		b.WriteString("  kind: " + o.StateBackend + "\n")
		// Config keys are declared by each state backend (providerpolicy →
		// extensions/states), not hardcoded here.
		for _, line := range providerpolicy.StateBackendScaffold(o.StateBackend) {
			b.WriteString("  " + line.Key + ": " + line.Value + "\n")
		}
		b.WriteString("\n")
	}

	if o.SecretManager != "" {
		b.WriteString("extensions:\n")
		b.WriteString("  secretManagerPlugin: " + o.SecretManager + "\n")
		b.WriteString("\n")
	}

	b.WriteString("functions:\n")
	b.WriteString("  - name: handler\n")
	b.WriteString("    entry: " + handler + "\n")
	b.WriteString("    runtime: " + runtime + "\n")
	b.WriteString("    triggers:\n")
	b.WriteString(generateEventYAML(o.Template, ext))
	return b.String()
}

// jsScaffold reports whether the target language is plain JavaScript, the only
// case where a provider's handler-body/entry-file overrides (e.g. Cloudflare's
// worker.js) apply; TypeScript/Python/Go fall back to the generic scaffold.
func jsScaffold(o *initOpts) bool {
	return o.Lang == "js" || o.Lang == "node"
}

// providerRegionLine returns the `${env:...}` expression for the provider.region
// line, derived from the provider's own credential surface (the *_REGION var and
// its placeholder default). Providers without a region credential omit the line.
func providerRegionLine(provider string) string {
	for _, c := range providerpolicy.ProviderCredentials(provider) {
		if strings.HasSuffix(c.EnvKey, "_REGION") {
			if c.Placeholder != "" {
				return "${env:" + c.EnvKey + "," + c.Placeholder + "}"
			}
			return "${env:" + c.EnvKey + "}"
		}
	}
	return ""
}

func generateEventYAML(trigger, ext string) string {
	switch trigger {
	case planner.TriggerHTTP:
		return "      - type: http\n        path: /hello\n        method: GET\n"
	case planner.TriggerCron:
		return "      - type: cron\n        schedule: \"rate(5 minutes)\"\n"
	case planner.TriggerQueue:
		return "      - type: queue\n        queue: my-queue\n"
	case planner.TriggerStorage:
		return "      - type: storage\n        bucket: my-bucket\n        events:\n          - s3:ObjectCreated:*\n"
	case planner.TriggerEventBridge:
		return "      - type: eventbridge\n        pattern:\n          source: [\"my.app\"]\n"
	case planner.TriggerPubSub:
		return "      - type: pubsub\n        topic: my-topic\n"
	default:
		return "      - type: http\n        path: /hello\n        method: GET\n"
	}
}

// generateSampleHandler returns the handler file path and body for the project.
// Generic language×trigger bodies come from the shared scaffold generator
// (platform/generator/application, also used by `runfabric generate function`);
// a provider may override the file name and body for plain-JS scaffolds (e.g.
// Cloudflare Workers' worker.js), declared via its ProviderScaffold.
func generateSampleHandler(o *initOpts) (path, content string) {
	res, _ := scaffold.HandlerContent(o.Lang, o.Template)
	content = res.Content
	switch o.Lang {
	case "python", "go":
		path = "handler" + res.Ext
	default:
		path = "src/handler" + res.Ext
	}

	sc := providerpolicy.ProviderScaffold(o.Provider)
	if jsScaffold(o) {
		if sc.EntryFile != "" {
			path = sc.EntryFile
		}
		if sc.Sample != "" {
			content = sc.Sample
		}
	}
	return path, content
}

// generateGitignore returns .gitignore content for the given language.
func generateGitignore(lang string) string {
	common := "# RunFabric\n.runfabric/\n.env\n.env.*\n!.env.example\n*.log\n"
	switch lang {
	case "node", "ts", "js":
		return common + "\n# Node\nnode_modules/\nnpm-debug.log*\n.npm\ndist/\nbuild/\ncoverage/\n.nyc_output/\n"
	case "python":
		return common + "\n# Python\n__pycache__/\n*.py[cod]\n*$py.class\n.venv/\nvenv/\n*.egg-info/\n.eggs/\n.pytest_cache/\n.coverage\n"
	case "go":
		return common + "\n# Go\nbin/\n*.exe\nvendor/\n*.test\n"
	default:
		return common + "\n"
	}
}

// providerEnvLines returns .env.example lines for a provider, derived from the
// provider's own CredentialVars declaration so the scaffold can never drift
// from what the provider code actually reads. Optional vars are commented out.
func providerEnvLines(provider string) []string {
	creds := providerpolicy.ProviderCredentials(provider)
	lines := make([]string, 0, len(creds))
	for _, c := range creds {
		line := c.EnvKey + "=" + c.Placeholder
		if !c.Required {
			line = "# " + line
		}
		lines = append(lines, line)
	}
	return lines
}

// stateEnvLines returns .env.example lines for a state backend, derived from
// the backend's CredentialVars declaration (extensions/states); optional vars
// are commented out. s3/dynamodb reuse AWS_* from the provider section.
func stateEnvLines(stateBackend string) []string {
	creds := providerpolicy.StateBackendCredentials(stateBackend)
	lines := make([]string, 0, len(creds))
	for _, c := range creds {
		line := c.EnvKey + "=" + c.Placeholder
		if !c.Required {
			line = "# " + line
		}
		lines = append(lines, line)
	}
	return lines
}

func generateEnvExample(provider, stateBackend string) string {
	var b strings.Builder
	b.WriteString("# RunFabric — copy to .env and fill in values\n")
	b.WriteString("# Generated for provider: " + provider + ", state: " + stateBackend + "\n\n")
	// Provider and state declarations can share vars (e.g. GCP_ACCESS_TOKEN
	// for gcp-functions + gcs) — emit each env key once.
	seen := map[string]bool{}
	envKeyOf := func(line string) string {
		key := strings.TrimPrefix(line, "# ")
		if i := strings.Index(key, "="); i > 0 {
			return key[:i]
		}
		return key
	}
	if vars := providerEnvLines(provider); len(vars) > 0 {
		b.WriteString("# Provider\n")
		for _, v := range vars {
			seen[envKeyOf(v)] = true
			b.WriteString(v + "\n")
		}
		b.WriteString("\n")
	}
	if stateBackend != "local" {
		if vars := stateEnvLines(stateBackend); len(vars) > 0 {
			b.WriteString("# State backend\n")
			for _, v := range vars {
				if seen[envKeyOf(v)] {
					continue
				}
				b.WriteString(v + "\n")
			}
		}
	}
	b.WriteString("# Optional: RUNFABRIC_STAGE=dev\n")
	b.WriteString("# Optional: RUNFABRIC_REAL_DEPLOY=1\n")
	return b.String()
}

// generatePackageJSON returns package.json content for Node/JS/TS projects.
func generatePackageJSON(o *initOpts) string {
	name := o.Service
	if name == "" {
		name = "my-service"
	}

	scripts := map[string]string{}
	if o.Lang == "ts" {
		scripts["start"] = "node dist/handler.js"
		if o.WithBuildScript {
			scripts["build"] = "tsc"
			scripts["build:watch"] = "tsc --watch --preserveWatchOutput"
		}
	} else {
		scripts["start"] = "node src/handler.js"
	}
	if o.CallLocal {
		if o.Lang == "ts" && o.WithBuildScript {
			scripts["call:local"] = "concurrently npm:build:watch 'runfabric invoke local -c runfabric.yml --serve --watch'"
		} else {
			scripts["call:local"] = "runfabric invoke local -c runfabric.yml --serve --watch"
		}
	}

	// Runtime dependencies are optional and loaded dynamically by RunFabric.
	// @runfabric/sdk and provider adapters can be added when published and needed.
	deps := map[string]string{}

	pkg := map[string]any{
		"name":         name,
		"version":      "0.1.0",
		"private":      true,
		"scripts":      scripts,
		"dependencies": deps,
	}
	if o.Lang == "ts" && o.WithBuildScript {
		pkg["devDependencies"] = map[string]string{
			"concurrently": "^9.0.1",
			"typescript":   "^5.0.0",
			"@types/node":  "^20.0.0",
		}
	}

	b, err := json.MarshalIndent(pkg, "", "  ")
	if err != nil {
		return "{\n  \"name\": \"my-service\",\n  \"private\": true\n}\n"
	}
	return string(b) + "\n"
}

func isNodeLang(lang string) bool {
	switch lang {
	case "js", "ts", "node":
		return true
	default:
		return false
	}
}

func isValidPackageManager(pm string) bool {
	switch strings.ToLower(strings.TrimSpace(pm)) {
	case "npm", "pnpm", "yarn", "bun":
		return true
	default:
		return false
	}
}

func packageInstallCommand(pm string) string {
	switch pm {
	case "pnpm":
		return "pnpm install"
	case "yarn":
		return "yarn install"
	case "bun":
		return "bun install"
	default:
		return "npm install"
	}
}

func packageRunCommand(pm, script string) string {
	switch pm {
	case "yarn":
		return "yarn " + script
	case "bun":
		return "bun run " + script
	default:
		return pm + " run " + script
	}
}

func installNodeDependencies(dir, pm string) error {
	cmdStr := packageInstallCommand(pm)
	parts := strings.Fields(cmdStr)
	if len(parts) == 0 {
		return nil
	}
	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s failed: %w", cmdStr, err)
	}
	return nil
}

func runNodeScript(dir, pm, script string) error {
	cmdStr := packageRunCommand(pm, script)
	parts := strings.Fields(cmdStr)
	if len(parts) == 0 {
		return nil
	}
	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s failed: %w", cmdStr, err)
	}
	return nil
}

// generateTsconfig returns tsconfig.json for TypeScript projects.
func generateTsconfig(o *initOpts) string {
	return `{
  "compilerOptions": {
    "target": "ES2020",
    "module": "commonjs",
    "lib": ["ES2020"],
    "outDir": "dist",
    "rootDir": "src",
    "strict": true,
    "esModuleInterop": true,
    "skipLibCheck": true,
    "declaration": true,
    "sourceMap": true
  },
  "include": ["src/**/*"],
  "exclude": ["node_modules", "dist"]
}
`
}

// generateREADME returns README.md content for the scaffolded project.
func generateREADME(o *initOpts) string {
	service := o.Service
	if service == "" {
		service = "my-service"
	}
	var b strings.Builder
	b.WriteString("# " + service + "\n\n")
	b.WriteString("RunFabric project — provider: **" + o.Provider + "**, trigger: **" + o.Template + "**.\n\n")
	b.WriteString("## Prerequisites\n\n")
	b.WriteString("- [RunFabric CLI](https://github.com/runfabric/runfabric) installed\n")
	switch o.Lang {
	case "ts", "js", "node":
		b.WriteString("- Node.js 18+\n")
		b.WriteString("- `npm install` (or your package manager)\n")
		if o.Lang == "ts" {
			b.WriteString("- For deploy: run `npm run build` so `dist/` is produced\n")
		}
	case "python":
		b.WriteString("- Python 3.11+\n")
	case "go":
		b.WriteString("- Go 1.21+\n")
	}
	b.WriteString("\n## Quick start\n\n")
	b.WriteString("1. Copy `.env.example` to `.env` and set credentials for " + o.Provider + ".\n")
	b.WriteString("2. Check config: `runfabric doctor --config runfabric.yml --stage dev`\n")
	if o.Lang == "ts" {
		b.WriteString("3. Build: `npm run build`\n")
		b.WriteString("4. Deploy: `runfabric deploy --config runfabric.yml --stage dev`\n")
	} else {
		b.WriteString("3. Deploy: `runfabric deploy --config runfabric.yml --stage dev`\n")
	}
	b.WriteString("\n## Config\n\n")
	b.WriteString("- **runfabric.yml** — service, provider, functions, events\n")
	switch o.Lang {
	case "ts":
		b.WriteString("- Handler: `src/handler.ts` (compiled to `dist/handler.js` for deploy)\n")
	case "js", "node":
		b.WriteString("- Handler: `src/handler.js`\n")
	case "python":
		b.WriteString("- Handler: `handler.py`\n")
	case "go":
		b.WriteString("- Handler: `handler.go`\n")
	default:
		b.WriteString("- Handler: see runfabric.yml\n")
	}
	return b.String()
}

// generateGitHubActionsWorkflow returns a minimal .github/workflows/deploy.yml (doctor → plan → deploy on push).
func generateGitHubActionsWorkflow(o *initOpts) string {
	const tpl = `# Minimal RunFabric CI: doctor → plan → deploy on push to main.
# Set RUNFABRIC_REAL_DEPLOY=1 and provider credentials (e.g. AWS_*) in repo secrets.
name: Deploy

on:
  push:
    branches: [main]

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.21'

      - name: Install RunFabric CLI
        run: go install github.com/runfabric/runfabric/cmd/runfabric@latest

      - name: Install dependencies
        run: |
          if [ -f package.json ]; then npm ci; fi

      - name: Build (if TypeScript)
        run: |
          if [ -f package.json ] && grep -q '"build"' package.json; then npm run build; fi

      - name: Doctor
        run: runfabric doctor --config runfabric.yml --stage prod
        env:
          RUNFABRIC_REAL_DEPLOY: "1"

      - name: Plan
        run: runfabric plan --config runfabric.yml --stage prod --json

      - name: Deploy
        run: runfabric deploy --config runfabric.yml --stage prod
        env:
          RUNFABRIC_REAL_DEPLOY: "1"
          # Add provider credentials via repo secrets, e.g.:
          # AWS_ACCESS_KEY_ID: ${{ secrets.AWS_ACCESS_KEY_ID }}
          # AWS_SECRET_ACCESS_KEY: ${{ secrets.AWS_SECRET_ACCESS_KEY }}
          # AWS_REGION: ${{ secrets.AWS_REGION }}
`
	return tpl
}
