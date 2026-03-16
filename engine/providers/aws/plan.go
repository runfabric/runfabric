package aws

import (
	"context"
	"time"

	"github.com/runfabric/runfabric/engine/internal/config"
	appErrs "github.com/runfabric/runfabric/engine/internal/errors"
	"github.com/runfabric/runfabric/engine/internal/planner"
	"github.com/runfabric/runfabric/engine/internal/providers"
	"github.com/runfabric/runfabric/engine/internal/runtime/build"
	"github.com/runfabric/runfabric/engine/internal/state"
)

func (p *Provider) Plan(cfg *config.Config, stage, root string) (*providers.PlanResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	clients, err := loadClients(ctx, cfg.Provider.Region)
	if err != nil {
		return nil, appErrs.Wrap(appErrs.CodeDeployFailed, "load aws config failed during plan", err)
	}

	artifactMap := map[string]providers.Artifact{}
	for fnName, fn := range cfg.Functions {
		configSig, err := buildConfigSignature(fn)
		if err != nil {
			return nil, appErrs.Wrap(appErrs.CodeDeployFailed, "build config signature failed: "+fnName, err)
		}

		artifact, err := build.PackageNodeFunction(root, fnName, fn, configSig)
		if err != nil {
			return nil, appErrs.Wrap(appErrs.CodeDeployFailed, "package function during plan failed: "+fnName, err)
		}
		artifactMap[fnName] = *artifact
	}

	actual, err := discoverActualState(ctx, clients, cfg, stage)
	if err != nil {
		return nil, appErrs.Wrap(appErrs.CodeDeployFailed, "discover actual state failed", err)
	}

	desired := desiredStateFromConfig(cfg, stage, artifactMap)
	plan := planner.Diff(desired, actual, cfg.Service, stage, cfg.Provider.Name)

	receipt, _ := state.Load(root, stage)
	receiptMap := buildReceiptFunctionMap(receipt)
	actualMap := actualLambdaMap(actual)

	for i := range plan.Actions {
		action := &plan.Actions[i]
		if action.Resource != planner.ResourceLambda {
			continue
		}

		fnConfigName := reverseFunctionName(cfg, stage, action.Name)
		if fnConfigName == "" {
			continue
		}

		fn, ok := cfg.Functions[fnConfigName]
		if !ok {
			continue
		}
		artifact := artifactMap[fnConfigName]

		desiredLambda := planner.DesiredLambda{
			Name:            functionName(cfg, stage, fnConfigName),
			Runtime:         fn.Runtime,
			Handler:         fn.Handler,
			Memory:          defaultInt(fn.Memory, 128),
			Timeout:         defaultInt(fn.Timeout, 10),
			CodeSHA256:      artifact.SHA256,
			ConfigSignature: artifact.ConfigSignature,
		}

		var actualLambdaPtr *planner.ActualLambda
		if actualLambda, ok := actualMap[action.Name]; ok {
			actualLambdaPtr = &actualLambda
		}

		changeSet := detectFunctionChange(fnConfigName, artifact, desiredLambda, actualLambdaPtr, receiptMap)

		switch {
		case changeSet.NeedsCreate:
			action.Type = planner.ActionCreate
			action.Description = "Create Lambda function"
			action.Metadata = mergeMap(action.Metadata, map[string]string{
				"reason": changeSet.Reason,
			})
		case changeSet.NeedsUpdate:
			action.Type = planner.ActionUpdate
			action.Description = "Update Lambda function"
			action.Metadata = mergeMap(action.Metadata, map[string]string{
				"reason":           changeSet.Reason,
				"remoteCodeChange": boolString(changeSet.RemoteCodeChange),
				"remoteCfgChange":  boolString(changeSet.RemoteCfgChange),
			})
			action.Changes = map[string][2]string{
				"desiredArtifactSha": {"remote/current", artifact.SHA256},
				"desiredConfigSig":   {"remote/current", artifact.ConfigSignature},
			}
		default:
			action.Type = planner.ActionNoop
			action.Description = "Lambda function unchanged"
			action.Metadata = mergeMap(action.Metadata, map[string]string{
				"reason": changeSet.Reason,
			})
		}
	}

	warnings := []string{}
	if actual.HTTPAPI == nil && desired.HTTPAPI != nil {
		warnings = append(warnings, "HTTP API does not exist yet and will be created")
	}

	return &providers.PlanResult{
		Provider: p.Name(),
		Plan:     plan,
		Warnings: warnings,
	}, nil
}

func reverseFunctionName(cfg *config.Config, stage, deployedName string) string {
	for fnName := range cfg.Functions {
		if functionName(cfg, stage, fnName) == deployedName {
			return fnName
		}
	}
	return ""
}

func mergeMap(a, b map[string]string) map[string]string {
	if a == nil && b == nil {
		return nil
	}
	out := map[string]string{}
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}

func boolString(v bool) string {
	if v {
		return "true"
	}
	return "false"
}
