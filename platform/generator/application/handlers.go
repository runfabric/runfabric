package application

import (
	"strings"

	planner "github.com/runfabric/runfabric/platform/planner/engine"
)

// HandlerResult is the generated handler file content and extension.
type HandlerResult struct {
	Content string // file body
	Ext     string // e.g. ".js", ".ts", ".py", ".go"
}

// HandlerContent returns handler file content and extension for the given lang and trigger.
// Lang: js, ts, node, python, go. Trigger: http, cron, queue (and optionally storage, eventbridge, pubsub).
func HandlerContent(lang, trigger string) (HandlerResult, bool) {
	lang = normalizeLang(lang)
	switch lang {
	case "node", "ts":
		content := handlerNodeTS(trigger, lang == "ts")
		ext := ".js"
		if lang == "ts" {
			ext = ".ts"
		}
		return HandlerResult{Content: content, Ext: ext}, true
	case "python":
		return HandlerResult{Content: handlerPython(trigger), Ext: ".py"}, true
	case "go":
		return HandlerResult{Content: handlerGo(trigger), Ext: ".go"}, true
	default:
		return HandlerResult{Content: handlerNodeTS(trigger, false), Ext: ".js"}, true
	}
}

func normalizeLang(lang string) string {
	if lang == "js" {
		return "node"
	}
	return lang
}

func handlerNodeTS(trigger string, isTS bool) string {
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

func handlerPython(trigger string) string {
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

func handlerGo(trigger string) string {
	switch trigger {
	case planner.TriggerHTTP:
		return "package main\n\nimport \"encoding/json\"\n\nfunc Handler(event map[string]interface{}, context interface{}) (map[string]interface{}, error) {\n\treturn map[string]interface{}{\n\t\t\"statusCode\": 200,\n\t\t\"body\":     `{\"message\":\"Hello from RunFabric\"}`,\n\t}, nil\n}\n"
	case planner.TriggerCron:
		return "package main\n\nfunc Handler(event map[string]interface{}, context interface{}) (map[string]interface{}, error) {\n\treturn map[string]interface{}{\"ok\": true}, nil\n}\n"
	default:
		return "package main\n\nfunc Handler(event map[string]interface{}, context interface{}) (map[string]interface{}, error) {\n\treturn map[string]interface{}{\"statusCode\": 200, \"body\": `{\"message\":\"Hello\"}`}, nil\n}\n"
	}
}

// BuildFunctionEntry returns a reference-format function entry suitable for configpatch.AddFunction.
// Route is for HTTP (e.g. "GET:/hello"), schedule for cron, queueName for queue.
// Method and path are parsed from route (METHOD:PATH); default GET and /<name>.
func BuildFunctionEntry(handlerPath, trigger, route, schedule, queueName string) map[string]any {
	entry := map[string]any{
		"entry": handlerPath,
	}
	var triggers []any
	switch trigger {
	case planner.TriggerHTTP:
		method, path := parseRoute(route)
		if path == "" {
			path = "/"
		}
		triggers = append(triggers, map[string]any{
			"type":   "http",
			"method": strings.ToUpper(method),
			"path":   path,
		})
	case planner.TriggerCron:
		s := schedule
		if s == "" {
			s = "rate(5 minutes)"
		}
		triggers = append(triggers, map[string]any{"type": "cron", "schedule": s})
	case planner.TriggerQueue:
		q := queueName
		if q == "" {
			q = "my-queue"
		}
		triggers = append(triggers, map[string]any{"type": "queue", "queue": q})
	default:
		triggers = append(triggers, map[string]any{"type": "http", "method": "GET", "path": "/"})
	}
	entry["triggers"] = triggers
	return entry
}

// parseRoute splits "METHOD:PATH" into method and path. Default method is GET.
func parseRoute(route string) (method, path string) {
	if route == "" {
		return "get", "/"
	}
	idx := strings.Index(route, ":")
	if idx <= 0 {
		return "get", route
	}
	method = strings.TrimSpace(route[:idx])
	path = strings.TrimSpace(route[idx+1:])
	if method == "" {
		method = "get"
	}
	if path == "" {
		path = "/"
	}
	return method, path
}

// BuildResourceEntry returns a YAML-friendly map for resources.<name> (database, cache, queue).
// typ is "database", "cache", or "queue"; connectionEnv is the env var name (e.g. DATABASE_URL).
func BuildResourceEntry(typ, connectionEnv string) map[string]any {
	entry := map[string]any{"type": typ}
	if connectionEnv != "" {
		entry["connectionStringEnv"] = connectionEnv
		entry["envVar"] = connectionEnv
	}
	return entry
}

// BuildAddonEntry returns a YAML-friendly map for addons.<name>. version can be empty.
func BuildAddonEntry(version string) map[string]any {
	entry := map[string]any{}
	if version != "" {
		entry["version"] = version
	}
	return entry
}

// BuildProviderOverrideEntry returns a YAML-friendly map for providerOverrides.<key>.
func BuildProviderOverrideEntry(providerName, runtime, region string) map[string]any {
	entry := map[string]any{
		"name": providerName,
	}
	if runtime != "" {
		entry["runtime"] = runtime
	}
	if region != "" {
		entry["region"] = region
	}
	return entry
}
