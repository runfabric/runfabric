package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/runfabric/runfabric/internal/planner"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const (
	asciiESC = 27  // Escape
	asciiETX = 3   // Ctrl+C
	maxESCSeq = 16 // max bytes to consume for an escape sequence
)

// initOpts holds init-specific flags (aligned with docs/site/command-reference.md).
type initOpts struct {
	Dir           string
	Template      string
	Provider      string
	StateBackend  string
	Lang          string
	Service       string
	PM            string
	SkipInstall   bool
	CallLocal     bool
	NoInteractive bool
}

var (
	triggerLabels = map[string]string{
		planner.TriggerHTTP:        "HTTP API",
		planner.TriggerCron:       "Scheduled (cron)",
		planner.TriggerQueue:      "Queue (SQS, etc.)",
		planner.TriggerStorage:    "Storage (S3, etc.)",
		planner.TriggerEventBridge: "EventBridge",
		planner.TriggerPubSub:     "Pub/Sub",
	}
	stateBackends = []string{"local", "s3", "gcs", "azblob", "postgres"}
	langs         = []string{"node", "ts", "python", "go"}
)

func newInitCmd(opts *GlobalOptions) *cobra.Command {
	initOpts := &initOpts{}

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Scaffold a new runfabric project",
		Long:  "Creates a new project with runfabric.yml and handler scaffolding. Use interactive mode (default) to select provider, trigger, language, and state backend, or pass flags for non-interactive.",
		RunE:  func(cmd *cobra.Command, args []string) error { return runInit(initOpts) },
	}

	cmd.Flags().StringVar(&initOpts.Dir, "dir", ".", "Target directory for the new project")
	cmd.Flags().StringVar(&initOpts.Template, "template", "", "Template/trigger: http|cron|queue|storage|eventbridge|pubsub")
	cmd.Flags().StringVar(&initOpts.Provider, "provider", "", "Provider (e.g. aws-lambda, gcp-functions)")
	cmd.Flags().StringVar(&initOpts.StateBackend, "state-backend", "", "State backend: local|s3|gcs|azblob|postgres (default: prompt in interactive mode)")
	cmd.Flags().StringVar(&initOpts.Lang, "lang", "", "Language: node|ts|python|go (default: prompt in interactive mode)")
	cmd.Flags().StringVar(&initOpts.Service, "service", "", "Service name (default: from --dir)")
	cmd.Flags().StringVar(&initOpts.PM, "pm", "npm", "Package manager: npm|pnpm|yarn|bun")
	cmd.Flags().BoolVar(&initOpts.SkipInstall, "skip-install", false, "Skip installing dependencies after scaffold")
	cmd.Flags().BoolVar(&initOpts.CallLocal, "call-local", false, "Add a script to run call-local after scaffold")
	cmd.Flags().BoolVar(&initOpts.NoInteractive, "no-interactive", false, "Disable interactive prompts; use flags only")

	return cmd
}

func runInit(o *initOpts) error {
	dir, err := filepath.Abs(o.Dir)
	if err != nil {
		return err
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
		if o.StateBackend == "" {
			o.StateBackend = promptState()
		}
		if o.Service == "" {
			o.Service = promptLine("Service name", filepath.Base(dir))
		}
	}

	// Defaults when non-interactive
	if o.Service == "" {
		o.Service = filepath.Base(dir)
	}
	if o.Service == "" || o.Service == "." {
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
	fmt.Printf("Wrote %s\n", ymlPath)

	// Generate sample handler
	handlerPath, handlerContent := generateSampleHandler(o)
	fullPath := filepath.Join(dir, handlerPath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return fmt.Errorf("create handler dir: %w", err)
	}
	if err := os.WriteFile(fullPath, []byte(handlerContent), 0o644); err != nil {
		return fmt.Errorf("write handler: %w", err)
	}
	fmt.Printf("Wrote %s\n", fullPath)

	fmt.Printf("\nProject ready in %s\n", dir)
	fmt.Printf("  provider: %s  trigger: %s  lang: %s  state: %s\n", o.Provider, o.Template, o.Lang, o.StateBackend)
	fmt.Printf("  Next: cd %s && runfabric doctor --config runfabric.yml --stage dev\n", filepath.Base(dir))
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
	fmt.Printf("%s [%s]: ", msg, defaultVal)
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
	b.WriteString("  handler:\n")
	b.WriteString("    handler: " + handler + "\n")
	b.WriteString("    memory: 128\n")
	b.WriteString("    timeout: 10\n")
	b.WriteString("    events:\n")
	b.WriteString(generateEventYAML(o.Template, ext))
	return b.String()
}

func generateEventYAML(trigger, ext string) string {
	switch trigger {
	case planner.TriggerHTTP:
		return "      - http:\n          path: /hello\n          method: get\n"
	case planner.TriggerCron:
		return "      - cron: \"rate(5 minutes)\"\n"
	case planner.TriggerQueue:
		return "      - queue:\n          queue: my-queue\n"
	case planner.TriggerStorage:
		return "      - storage:\n          bucket: my-bucket\n          events:\n            - s3:ObjectCreated:*\n"
	case planner.TriggerEventBridge:
		return "      - eventbridge:\n          pattern:\n            source: [\"my.app\"]\n"
	case planner.TriggerPubSub:
		return "      - pubsub:\n          topic: my-topic\n"
	default:
		return "      - http:\n          path: /hello\n          method: get\n"
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
	_ = isTS
	switch trigger {
	case planner.TriggerHTTP:
		return "exports.handler = async (event, context) => {\n  return {\n    statusCode: 200,\n    body: JSON.stringify({ message: \"Hello from RunFabric\", trigger: \"http\" }),\n  };\n};\n"
	case planner.TriggerCron:
		return "exports.handler = async (event, context) => {\n  console.log('Cron triggered at', new Date().toISOString());\n  return { ok: true };\n};\n"
	case planner.TriggerQueue:
		return "exports.handler = async (event, context) => {\n  const records = event.Records || event.records || [];\n  for (const r of records) {\n    console.log('Queue message:', r.body || r);\n  }\n  return { ok: true };\n};\n"
	case planner.TriggerStorage:
		return "exports.handler = async (event, context) => {\n  const records = event.Records || [];\n  for (const r of records) {\n    console.log('Object:', r.s3?.object?.key || r);\n  }\n  return { ok: true };\n};\n"
	case planner.TriggerEventBridge, planner.TriggerPubSub:
		return "exports.handler = async (event, context) => {\n  console.log('Event:', JSON.stringify(event, null, 2));\n  return { ok: true };\n};\n"
	default:
		return "exports.handler = async (event, context) => {\n  return { statusCode: 200, body: JSON.stringify({ message: \"Hello\" }) };\n};\n"
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
