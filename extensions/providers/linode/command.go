package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strings"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

func (p *plugin) executeOperation(ctx context.Context, root string, cfg sdkprovider.Config, operation, stage, function string, payload []byte) ([]byte, error) {
	command := p.resolveCommand(cfg, operation)
	if command == "" {
		return nil, fmt.Errorf("no %s command configured: set %s or config.commands.%s", operation, commandEnvForOperation(operation), operation)
	}
	service, functions, _, err := p.inspectConfig(cfg)
	if err != nil {
		return nil, err
	}
	selectedFunction := function
	selectedSpec := functionSpec{}
	if selectedFunction == "" && len(functions) == 1 {
		selectedFunction = functions[0].Name
	}
	for _, fn := range functions {
		if fn.Name == selectedFunction {
			selectedSpec = fn
			break
		}
	}
	artifactPath := p.resolveArtifactPath(root, selectedSpec)
	env := append(os.Environ(),
		"RUNFABRIC_PROVIDER="+p.provider,
		"RUNFABRIC_SERVICE="+service,
		"RUNFABRIC_STAGE="+stage,
		"RUNFABRIC_ROOT="+root,
		"RUNFABRIC_FUNCTION="+selectedFunction,
		"RUNFABRIC_RUNTIME="+selectedSpec.Runtime,
		"RUNFABRIC_ENTRY="+selectedSpec.Entry,
		"RUNFABRIC_ARTIFACT_PATH="+artifactPath,
		"RUNFABRIC_ARTIFACT_DIR="+pathDir(artifactPath),
		"RUNFABRIC_ARTIFACT_BASENAME="+pathBase(artifactPath),
		"RUNFABRIC_PAYLOAD_BASE64="+base64.StdEncoding.EncodeToString(payload),
	)
	if token, _ := p.resolveToken(cfg); token != "" {
		env = append(env, "LINODE_TOKEN="+token)
	}
	if appID := strings.TrimSpace(asString(cfg["appID"])); appID != "" {
		env = append(env, "RUNFABRIC_LINODE_APP_ID="+appID)
	}
	out, err := p.runCommand(ctx, root, command, env)
	if err != nil {
		trimmed := strings.TrimSpace(string(out))
		if trimmed == "" {
			return nil, fmt.Errorf("%s command failed: %w", operation, err)
		}
		return nil, fmt.Errorf("%s command failed: %w: %s", operation, err, trimmed)
	}
	return out, nil
}

func defaultCommandRunner(ctx context.Context, cwd, command string, env []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "/bin/sh", "-lc", command)
	if strings.TrimSpace(cwd) != "" {
		cmd.Dir = cwd
	}
	cmd.Env = env
	return cmd.CombinedOutput()
}

func (p *plugin) resolveArtifactPath(root string, fn functionSpec) string {
	if strings.TrimSpace(fn.Artifact) != "" {
		return joinRoot(root, fn.Artifact)
	}
	if strings.TrimSpace(fn.Name) == "" {
		return ""
	}
	for _, candidate := range []string{
		pathJoin(root, ".runfabric", fn.Name+".zip"),
		pathJoin(root, "dist", fn.Name+".zip"),
		pathJoin(root, "build", fn.Name+".zip"),
	} {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}
