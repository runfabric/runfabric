package project

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/runfabric/runfabric/internal/cli/common"
)

// ScaffoldOptions are the inputs to Scaffold — the generation-relevant subset of
// initOpts, with no filesystem/prompt/install flags. Empty fields take the same
// defaults `runfabric init` uses (provider aws-lambda, lang ts, state local,
// template http, service my-service).
type ScaffoldOptions struct {
	Provider      string
	Template      string // trigger: http|cron|queue|storage|eventbridge|pubsub
	Lang          string // js|ts|node|python|go
	StateBackend  string // local|s3|gcs|azblob|postgres|dynamodb|sqlite
	Service       string
	SecretManager string
	PM            string // npm|pnpm|yarn|bun
	WithBuild     bool
	WithCI        string // "" | github-actions
}

// ScaffoldFile is one generated file: a slash-separated path relative to the
// project root and its content.
type ScaffoldFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// ScaffoldResult is the generated starter project as in-memory files plus the
// resolved functions[].entry and runtime id (handy for callers that persist a
// structured config, like the PaaS).
type ScaffoldResult struct {
	Files   []ScaffoldFile `json:"files"`
	Entry   string         `json:"entry"`
	Runtime string         `json:"runtime"`
}

// Scaffold generates a starter project's files from provider/state metadata with
// no filesystem or prompt side effects. It is the reusable core of `runfabric
// init`, shared with the daemon's POST /scaffold route and `init --json`.
func Scaffold(o ScaffoldOptions) (*ScaffoldResult, error) {
	return scaffoldProject(&initOpts{
		Provider:        o.Provider,
		Template:        o.Template,
		Lang:            o.Lang,
		StateBackend:    o.StateBackend,
		Service:         o.Service,
		SecretManager:   o.SecretManager,
		PM:              o.PM,
		WithBuildScript: o.WithBuild,
		WithCI:          o.WithCI,
	})
}

// runInitJSON generates the project as JSON (files map + entry/runtime + the
// resolved provider/lang/trigger/state) for API callers, without touching the
// filesystem or prompting. Backs `runfabric init --json`.
func runInitJSON(o *initOpts) error {
	res, err := scaffoldProject(o)
	if err != nil {
		return err
	}
	files := make(map[string]string, len(res.Files))
	for _, f := range res.Files {
		files[f.Path] = f.Content
	}
	return common.PrintJSONSuccess("init", map[string]any{
		"service":      o.Service,
		"provider":     o.Provider,
		"template":     o.Template,
		"lang":         o.Lang,
		"stateBackend": o.StateBackend,
		"entry":        res.Entry,
		"runtime":      res.Runtime,
		"files":        files,
	})
}

// applyScaffoldDefaults fills the same defaults `runfabric init` applies and
// validates provider/trigger/lang — no filesystem or prompt I/O. (The dir-based
// service default stays in runInit, which knows the target directory.)
func applyScaffoldDefaults(o *initOpts) error {
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
	if o.Template == "api" || o.Template == "worker" {
		return fmt.Errorf("template %q is no longer supported; use http, cron, queue, storage, eventbridge, or pubsub", o.Template)
	}
	if o.Provider == "" {
		o.Provider = "aws-lambda"
	}
	normalizedSecretManager, err := normalizeSecretManagerPlugin(o.SecretManager)
	if err != nil {
		return err
	}
	o.SecretManager = normalizedSecretManager
	if isNodeLang(o.Lang) {
		o.PM = strings.ToLower(strings.TrimSpace(o.PM))
		if o.PM == "" {
			o.PM = "npm"
		}
		if !isValidPackageManager(o.PM) {
			return fmt.Errorf("unsupported --pm %q; use npm, pnpm, yarn, or bun", o.PM)
		}
	}
	if !initProviderSupportsTrigger(o.Provider, o.Template) {
		return fmt.Errorf("provider %q does not support trigger %q (see Trigger Capability Matrix)", o.Provider, o.Template)
	}
	switch o.Lang {
	case "node", "ts", "js", "python", "go":
	default:
		return fmt.Errorf("unsupported --lang %q; use node, ts, js, python, or go", o.Lang)
	}
	return nil
}

// scaffoldProject applies defaults+validation, then generates the project files in
// the canonical order runInit writes them. Pure — no filesystem writes.
func scaffoldProject(o *initOpts) (*ScaffoldResult, error) {
	if err := applyScaffoldDefaults(o); err != nil {
		return nil, err
	}
	runtime, entry, _ := resolveRuntimeEntryExt(o)

	files := make([]ScaffoldFile, 0, 8)
	files = append(files, ScaffoldFile{Path: "runfabric.yml", Content: generateRunfabricYAML(o)})
	handlerPath, handlerContent := generateSampleHandler(o)
	files = append(files, ScaffoldFile{Path: filepath.ToSlash(handlerPath), Content: handlerContent})
	files = append(files, ScaffoldFile{Path: ".gitignore", Content: generateGitignore(o.Lang)})
	files = append(files, ScaffoldFile{Path: ".env.example", Content: generateEnvExample(o.Provider, o.StateBackend)})
	if o.Lang == "js" || o.Lang == "ts" || o.Lang == "node" {
		files = append(files, ScaffoldFile{Path: "package.json", Content: generatePackageJSON(o)})
	}
	if o.Lang == "ts" {
		files = append(files, ScaffoldFile{Path: "tsconfig.json", Content: generateTsconfig(o)})
	}
	files = append(files, ScaffoldFile{Path: "README.md", Content: generateREADME(o)})
	if o.WithCI == "github-actions" {
		files = append(files, ScaffoldFile{Path: ".github/workflows/deploy.yml", Content: generateGitHubActionsWorkflow(o)})
	}
	return &ScaffoldResult{Files: files, Entry: entry, Runtime: runtime}, nil
}
