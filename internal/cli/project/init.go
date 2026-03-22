package project

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/runfabric/runfabric/internal/cli/common"
	planner "github.com/runfabric/runfabric/platform/core/planner/engine"
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
)

func newInitCmd(opts *GlobalOptions) *cobra.Command {
	initOpts := &initOpts{}

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Scaffold a new runfabric project",
		Long:  "Creates a new project with runfabric.yml and handler scaffolding. Use interactive mode (default) to select provider, trigger, language, and state backend, or pass flags for non-interactive.",
		RunE: func(c *cobra.Command, args []string) error {
			initOpts.NoInteractive = initOpts.NoInteractive || opts.NonInteractive
			return runInit(initOpts)
		},
	}

	cmd.Flags().StringVar(&initOpts.Dir, "dir", "", "Target directory (default: create folder named after service)")
	cmd.Flags().StringVar(&initOpts.Template, "template", "", "Template/trigger: http|cron|queue|storage|eventbridge|pubsub")
	cmd.Flags().StringVar(&initOpts.Provider, "provider", "", "Provider (e.g. aws-lambda, gcp-functions)")
	cmd.Flags().StringVar(&initOpts.StateBackend, "state-backend", "", "State backend: local|s3|gcs|azblob|postgres (default: prompt in interactive mode)")
	cmd.Flags().StringVar(&initOpts.Lang, "lang", "", "Language: js|ts|python|go (default: prompt in interactive mode)")
	cmd.Flags().StringVar(&initOpts.Service, "service", "", "Service name (default: from --dir)")
	cmd.Flags().StringVar(&initOpts.PM, "pm", "npm", "Package manager: npm|pnpm|yarn|bun")
	cmd.Flags().BoolVar(&initOpts.SkipInstall, "skip-install", false, "Skip installing dependencies after scaffold")
	cmd.Flags().BoolVar(&initOpts.CallLocal, "call-local", false, "Add a script to run call-local after scaffold")
	cmd.Flags().BoolVar(&initOpts.NoInteractive, "no-interactive", false, "Disable interactive prompts; use flags only")
	cmd.Flags().BoolVar(&initOpts.WithBuildScript, "with-build", false, "Add package.json and (for TypeScript) tsconfig.json with build script (tsc)")
	cmd.Flags().StringVar(&initOpts.WithCI, "with-ci", "", "Add CI workflow: github-actions (doctor → plan → deploy on push)")

	return cmd
}

func runInit(o *initOpts) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
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
		if o.Provider == "" {
			o.Provider = promptProvider()
		}
		if o.Provider != "" && o.Template == "" {
			o.Template = promptTrigger(o.Provider)
		}
		if o.Lang == "" {
			o.Lang = promptLang()
		}
		// When lang is ts, prompt for build script (tsconfig + tsc)
		if o.Lang == "ts" && !o.NoInteractive {
			idx := promptSelect("Add TypeScript build script (tsc)?", []string{"No", "Yes"}, 1)
			o.WithBuildScript = idx == 1
		}
		if o.StateBackend == "" {
			o.StateBackend = promptState()
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

	// Defaults when non-interactive
	if o.Service == "" {
		if dir != "" {
			o.Service = filepath.Base(dir)
		} else {
			o.Service = "my-service"
		}
	}
	if o.Service == "" || o.Service == "." || o.Service == "engine" {
		o.Service = "my-service"
	}
	if o.Lang == "" {
		o.Lang = "ts"
	}
	if o.StateBackend == "" {
		o.StateBackend = "local"
	}
	if o.Template == "" {
		o.Template = "http"
	}
	// Template aliases: api -> http, worker -> queue
	if o.Template == "api" {
		o.Template = "http"
	}
	if o.Template == "worker" {
		o.Template = "queue"
	}
	if o.Provider == "" {
		o.Provider = "aws-lambda"
	}

	// Validate trigger for provider
	if !planner.SupportsTrigger(o.Provider, o.Template) {
		return fmt.Errorf("provider %q does not support trigger %q (see Trigger Capability Matrix)", o.Provider, o.Template)
	}
	switch o.Lang {
	case "node", "ts", "js", "python", "go":
	default:
		return fmt.Errorf("unsupported --lang %q; use node, ts, js, python, or go", o.Lang)
	}

	// If --dir was not passed, create a folder named after the service
	if o.Dir == "" {
		dir = filepath.Join(cwd, o.Service)
	}

	// Create directory
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create dir %s: %w", dir, err)
	}
	srcDir := filepath.Join(dir, "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		return fmt.Errorf("create src dir: %w", err)
	}

	// Generate runfabric.yml
	yml := generateRunfabricYAML(o)
	ymlPath := filepath.Join(dir, "runfabric.yml")
	if err := os.WriteFile(ymlPath, []byte(yml), 0o644); err != nil {
		return fmt.Errorf("write runfabric.yml: %w", err)
	}
	common.InitWrote("runfabric.yml")

	// Generate sample handler
	handlerPath, handlerContent := generateSampleHandler(o)
	fullPath := filepath.Join(dir, handlerPath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return fmt.Errorf("create handler dir: %w", err)
	}
	if err := os.WriteFile(fullPath, []byte(handlerContent), 0o644); err != nil {
		return fmt.Errorf("write handler: %w", err)
	}
	common.InitWrote(filepath.ToSlash(handlerPath))

	// .gitignore (based on lang)
	gitignorePath := filepath.Join(dir, ".gitignore")
	gitignoreContent := generateGitignore(o.Lang)
	if err := os.WriteFile(gitignorePath, []byte(gitignoreContent), 0o644); err != nil {
		return fmt.Errorf("write .gitignore: %w", err)
	}
	common.InitWrote(".gitignore")

	// .env.example (based on provider and state backend)
	envExamplePath := filepath.Join(dir, ".env.example")
	envExampleContent := generateEnvExample(o.Provider, o.StateBackend)
	if err := os.WriteFile(envExamplePath, []byte(envExampleContent), 0o644); err != nil {
		return fmt.Errorf("write .env.example: %w", err)
	}
	common.InitWrote(".env.example")

	// package.json for Node/JS/TS
	if o.Lang == "js" || o.Lang == "ts" || o.Lang == "node" {
		pkgPath := filepath.Join(dir, "package.json")
		pkgContent := generatePackageJSON(o)
		if err := os.WriteFile(pkgPath, []byte(pkgContent), 0o644); err != nil {
			return fmt.Errorf("write package.json: %w", err)
		}
		common.InitWrote("package.json")
	}
	// tsconfig.json for TypeScript (and when with-build so build script works)
	if o.Lang == "ts" {
		tsPath := filepath.Join(dir, "tsconfig.json")
		tsContent := generateTsconfig(o)
		if err := os.WriteFile(tsPath, []byte(tsContent), 0o644); err != nil {
			return fmt.Errorf("write tsconfig.json: %w", err)
		}
		common.InitWrote("tsconfig.json")
	}

	// README.md
	readmePath := filepath.Join(dir, "README.md")
	readmeContent := generateREADME(o)
	if err := os.WriteFile(readmePath, []byte(readmeContent), 0o644); err != nil {
		return fmt.Errorf("write README.md: %w", err)
	}
	common.InitWrote("README.md")

	// Optional: GitHub Actions workflow (doctor → plan → deploy on push)
	if o.WithCI == "github-actions" {
		workflowDir := filepath.Join(dir, ".github", "workflows")
		if err := os.MkdirAll(workflowDir, 0o755); err != nil {
			return fmt.Errorf("create .github/workflows: %w", err)
		}
		workflowPath := filepath.Join(workflowDir, "deploy.yml")
		workflowContent := generateGitHubActionsWorkflow(o)
		if err := os.WriteFile(workflowPath, []byte(workflowContent), 0o644); err != nil {
			return fmt.Errorf("write .github/workflows/deploy.yml: %w", err)
		}
		common.InitWrote(".github/workflows/deploy.yml")
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
	if projectDirLabel == "." {
		common.InitNext("runfabric doctor --config runfabric.yml --stage dev")
	} else {
		common.InitNext("cd " + projectDirLabel + " && runfabric doctor --config runfabric.yml --stage dev")
	}
	return nil
}

func promptProvider() string {
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
	triggers := planner.SupportedTriggers(provider)
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

func generateRunfabricYAML(o *initOpts) string {
	runtime := "nodejs20.x"
	if o.Lang == "python" {
		runtime = "python3.11"
	}
	if o.Lang == "go" {
		runtime = "go1.x"
	}
	handler := "src/handler.handler"
	ext := ".js"
	if o.Lang == "ts" {
		// TypeScript build outputs to dist/handler.js; deploy uses compiled output
		handler = "dist/handler.handler"
		ext = ".ts"
	}
	if o.Lang == "python" {
		handler = "handler.handler"
		ext = ".py"
	}
	if o.Lang == "go" {
		handler = "handler"
		ext = ".go"
	}

	var b strings.Builder
	b.WriteString("# RunFabric config — generated by runfabric init\n")
	// Provider-specific comment (template set per trigger × provider × language)
	b.WriteString(providerComment(o.Provider) + "\n")
	b.WriteString("service: " + yamlQuoted(o.Service) + "\n\n")
	b.WriteString("provider:\n")
	b.WriteString("  name: " + o.Provider + "\n")
	b.WriteString("  runtime: " + runtime + "\n")
	b.WriteString("  region: ${env:AWS_REGION,us-east-1}\n\n")

	if o.StateBackend != "local" {
		b.WriteString("backend:\n")
		b.WriteString("  kind: " + o.StateBackend + "\n")
		if o.StateBackend == "s3" {
			b.WriteString("  s3Bucket: ${env:RUNFABRIC_S3_BUCKET}\n")
			b.WriteString("  s3Prefix: runfabric/dev\n")
			b.WriteString("  lockTable: ${env:RUNFABRIC_DYNAMODB_TABLE}\n")
		}
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

// providerComment returns a short provider-specific comment for the generated runfabric.yml (template set per provider).
func providerComment(provider string) string {
	switch provider {
	case "aws-lambda":
		return "# Provider: aws-lambda — set AWS_REGION; optional backend: s3 + DynamoDB for state"
	case "gcp-functions":
		return "# Provider: gcp-functions — set GCP_PROJECT_ID; supports http, cron, queue, storage, pubsub"
	case "azure-functions":
		return "# Provider: azure-functions — set AZURE_*; supports http, cron, queue, storage"
	case "cloudflare-workers":
		return "# Provider: cloudflare-workers — set CLOUDFLARE_*; supports http, cron"
	case "vercel", "netlify":
		return "# Provider: " + provider + " — set provider token; supports http, cron"
	case "fly-machines":
		return "# Provider: fly-machines — set FLY_*; http only"
	case "kubernetes":
		return "# Provider: kubernetes — set KUBECONFIG; supports http, cron"
	case "alibaba-fc", "digitalocean-functions", "ibm-openwhisk":
		return "# Provider: " + provider + " — see docs for credentials and trigger support"
	default:
		return ""
	}
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

func generateSampleHandler(o *initOpts) (path, content string) {
	trigger := o.Template
	lang := o.Lang
	if lang == "js" {
		lang = "node"
	}

	switch lang {
	case "node", "ts":
		return "src/handler" + map[bool]string{true: ".ts", false: ".js"}[lang == "ts"], sampleHandlerNodeTS(trigger, lang == "ts")
	case "python":
		return "handler.py", sampleHandlerPython(trigger)
	case "go":
		return "handler.go", sampleHandlerGo(trigger)
	default:
		return "src/handler.js", sampleHandlerNodeTS(trigger, false)
	}
}

func sampleHandlerNodeTS(trigger string, isTS bool) string {
	sig := "(event, context)"
	if isTS {
		sig = "(event: any, context: any)"
	}
	switch trigger {
	case planner.TriggerHTTP:
		return "exports.handler = async" + sig + " => {\n  return {\n    statusCode: 200,\n    body: JSON.stringify({ message: \"Hello from RunFabric\", trigger: \"http\" }),\n  };\n};\n"
	case planner.TriggerCron:
		return "exports.handler = async" + sig + " => {\n  console.log('Cron triggered at', new Date().toISOString());\n  return { ok: true };\n};\n"
	case planner.TriggerQueue:
		return "exports.handler = async" + sig + " => {\n  const records = event.Records || event.records || [];\n  for (const r of records) {\n    console.log('Queue message:', r.body || r);\n  }\n  return { ok: true };\n};\n"
	case planner.TriggerStorage:
		return "exports.handler = async" + sig + " => {\n  const records = event.Records || [];\n  for (const r of records) {\n    console.log('Object:', r.s3?.object?.key || r);\n  }\n  return { ok: true };\n};\n"
	case planner.TriggerEventBridge, planner.TriggerPubSub:
		return "exports.handler = async" + sig + " => {\n  console.log('Event:', JSON.stringify(event, null, 2));\n  return { ok: true };\n};\n"
	default:
		return "exports.handler = async" + sig + " => {\n  return { statusCode: 200, body: JSON.stringify({ message: \"Hello\" }) };\n};\n"
	}
}

func sampleHandlerPython(trigger string) string {
	switch trigger {
	case planner.TriggerHTTP:
		return "def handler(event, context):\n    return {\"statusCode\": 200, \"body\": '{\"message\": \"Hello from RunFabric\"}'}\n"
	case planner.TriggerCron:
		return "def handler(event, context):\n    print(\"Cron triggered\")\n    return {\"ok\": True}\n"
	case planner.TriggerQueue, planner.TriggerStorage, planner.TriggerEventBridge, planner.TriggerPubSub:
		return "def handler(event, context):\n    print(\"Event:\", event)\n    return {\"ok\": True}\n"
	default:
		return "def handler(event, context):\n    return {\"statusCode\": 200, \"body\": '{\"message\": \"Hello\"}'}\n"
	}
}

func sampleHandlerGo(trigger string) string {
	switch trigger {
	case planner.TriggerHTTP:
		return "package main\n\nimport \"encoding/json\"\n\nfunc Handler(event map[string]interface{}, context interface{}) (map[string]interface{}, error) {\n\treturn map[string]interface{}{\n\t\t\"statusCode\": 200,\n\t\t\"body\":     `{\"message\":\"Hello from RunFabric\"}`,\n\t}, nil\n}\n"
	case planner.TriggerCron:
		return "package main\n\nfunc Handler(event map[string]interface{}, context interface{}) (map[string]interface{}, error) {\n\treturn map[string]interface{}{\"ok\": true}, nil\n}\n"
	default:
		return "package main\n\nfunc Handler(event map[string]interface{}, context interface{}) (map[string]interface{}, error) {\n\treturn map[string]interface{}{\"statusCode\": 200, \"body\": `{\"message\":\"Hello\"}`}, nil\n}\n"
	}
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

// providerEnvVars returns env var names (with optional placeholder) for .env.example for a provider.
var providerEnvVars = map[string][]string{
	"aws-lambda":             {"AWS_ACCESS_KEY_ID=", "AWS_SECRET_ACCESS_KEY=", "AWS_REGION=us-east-1"},
	"gcp-functions":          {"GCP_PROJECT_ID=", "GCP_SERVICE_ACCOUNT_KEY="},
	"azure-functions":        {"AZURE_TENANT_ID=", "AZURE_CLIENT_ID=", "AZURE_CLIENT_SECRET=", "AZURE_SUBSCRIPTION_ID=", "AZURE_RESOURCE_GROUP="},
	"kubernetes":             {"KUBECONFIG=", "KUBE_CONTEXT=", "KUBE_NAMESPACE="},
	"cloudflare-workers":     {"CLOUDFLARE_API_TOKEN=", "CLOUDFLARE_ACCOUNT_ID="},
	"vercel":                 {"VERCEL_TOKEN=", "VERCEL_ORG_ID=", "VERCEL_PROJECT_ID="},
	"netlify":                {"NETLIFY_AUTH_TOKEN=", "NETLIFY_SITE_ID="},
	"alibaba-fc":             {"ALICLOUD_ACCESS_KEY_ID=", "ALICLOUD_ACCESS_KEY_SECRET=", "ALICLOUD_REGION="},
	"digitalocean-functions": {"DIGITALOCEAN_ACCESS_TOKEN=", "DIGITALOCEAN_NAMESPACE="},
	"fly-machines":           {"FLY_API_TOKEN=", "FLY_APP_NAME="},
	"ibm-openwhisk":          {"IBM_CLOUD_API_KEY=", "IBM_CLOUD_REGION=", "IBM_CLOUD_NAMESPACE="},
}

// stateEnvVars returns env var lines for .env.example for a state backend.
var stateEnvVars = map[string][]string{
	"local":    {},
	"postgres": {"RUNFABRIC_STATE_POSTGRES_URL=postgres://user:pass@localhost:5432/runfabric"},
	"s3":       {"RUNFABRIC_S3_BUCKET=", "RUNFABRIC_DYNAMODB_TABLE=", "# Uses AWS_* from provider if same account"},
	"gcs":      {"GOOGLE_APPLICATION_CREDENTIALS=/path/to/service-account.json"},
	"azblob":   {"AZURE_STORAGE_CONNECTION_STRING=", "# or AZURE_STORAGE_ACCOUNT= + AZURE_STORAGE_KEY="},
}

func generateEnvExample(provider, stateBackend string) string {
	var b strings.Builder
	b.WriteString("# RunFabric — copy to .env and fill in values\n")
	b.WriteString("# Generated for provider: " + provider + ", state: " + stateBackend + "\n\n")
	if vars, ok := providerEnvVars[provider]; ok {
		b.WriteString("# Provider\n")
		for _, v := range vars {
			b.WriteString(v + "\n")
		}
		b.WriteString("\n")
	}
	if stateBackend != "local" {
		if vars, ok := stateEnvVars[stateBackend]; ok && len(vars) > 0 {
			b.WriteString("# State backend\n")
			for _, v := range vars {
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
	lang := o.Lang
	if lang == "js" {
		lang = "node"
	}
	name := o.Service
	if name == "" {
		name = "my-service"
	}
	// Escape for JSON: only need to escape " and \
	nameEsc := strings.ReplaceAll(name, "\\", "\\\\")
	nameEsc = strings.ReplaceAll(nameEsc, "\"", "\\\"")
	var b strings.Builder
	b.WriteString("{\n  \"name\": \"" + nameEsc + "\",\n  \"version\": \"0.1.0\",\n  \"private\": true,\n  \"scripts\": {\n")
	if o.Lang == "ts" {
		b.WriteString("    \"start\": \"node dist/handler.js\",\n    \"build\": \"tsc\"\n  }")
		if o.WithBuildScript {
			b.WriteString(",\n  \"devDependencies\": {\n    \"typescript\": \"^5.0.0\",\n    \"@types/node\": \"^20.0.0\"\n  }")
		}
	} else {
		b.WriteString("    \"start\": \"node src/handler.js\"")
		if o.WithBuildScript {
			b.WriteString(",\n    \"build\": \"echo 'No build step'\"")
		}
		b.WriteString("\n  }")
	}
	b.WriteString("\n}\n")
	return b.String()
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
