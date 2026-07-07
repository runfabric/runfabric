package main

import (
	"context"
	"fmt"
	"strings"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

func (p *plugin) Plan(ctx context.Context, req sdkprovider.PlanRequest) (*sdkprovider.PlanResult, error) {
	_ = ctx
	service, functions, warnings, err := p.inspectConfig(req.Config)
	if err != nil {
		return nil, err
	}
	actions := make([]map[string]any, 0, len(functions))
	for _, fn := range functions {
		actions = append(actions, map[string]any{
			"type":     "deploy-function",
			"function": fn.Name,
			"runtime":  fn.Runtime,
			"entry":    fn.Entry,
			"artifact": fn.Artifact,
			"triggers": append([]string(nil), fn.Triggers...),
		})
	}
	plan := map[string]any{
		"provider": p.provider,
		"service":  service,
		"stage":    req.Stage,
		"root":     req.Root,
		"actions":  actions,
		"commands": map[string]bool{
			"deploy": p.resolveCommand(req.Config, "deploy") != "",
			"remove": p.resolveCommand(req.Config, "remove") != "",
			"invoke": p.resolveCommand(req.Config, "invoke") != "" || p.resolveInvokeURL(req.Config, "") != "",
			"logs":   p.resolveCommand(req.Config, "logs") != "",
		},
	}
	if token, _ := p.resolveToken(req.Config); token == "" {
		warnings = append(warnings, fmt.Sprintf("set %s or config.token before running doctor", defaultTokenEnv))
	}
	if p.resolveCommand(req.Config, "deploy") == "" {
		warnings = append(warnings, fmt.Sprintf("set %s or config.commands.deploy to enable deployments", deployCommandEnv))
	}
	for _, fn := range functions {
		if strings.TrimSpace(fn.Artifact) == "" {
			warnings = append(warnings, fmt.Sprintf("function %s has no artifact configured; set artifact/outputPath or place a zip in dist/, build/, or .runfabric/", fn.Name))
		}
	}
	if p.resolveCommand(req.Config, "remove") == "" {
		warnings = append(warnings, fmt.Sprintf("set %s or config.commands.remove to enable removals", removeCommandEnv))
	}
	if p.resolveCommand(req.Config, "logs") == "" {
		warnings = append(warnings, fmt.Sprintf("set %s or config.commands.logs to enable log collection", logsCommandEnv))
	}
	if p.resolveCommand(req.Config, "invoke") == "" && p.resolveInvokeURL(req.Config, "") == "" {
		warnings = append(warnings, fmt.Sprintf("set %s, config.commands.invoke, or a function URL to enable invocation", invokeCommandEnv))
	}
	return &sdkprovider.PlanResult{Provider: p.provider, Plan: plan, Warnings: warnings}, nil
}
