package aws

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	sfnv2 "github.com/aws/aws-sdk-go-v2/service/sfn"
	providers "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"
	appErrs "github.com/runfabric/runfabric/platform/core/model/errors"
	"github.com/runfabric/runfabric/platform/state/locking"
)

func (p *Provider) Remove(ctx context.Context, req providers.RemoveRequest) (*providers.RemoveResult, error) {
	cfg := req.Config
	stage := req.Stage
	root := req.Root
	metadata := map[string]string{
		"remove:lock": "acquired",
	}
	_ = metadata

	lock, err := locking.Acquire(root, cfg.Service, stage)
	if err != nil {
		return nil, appErrs.Wrap(appErrs.CodeRemoveFailed, "acquire remove lock failed", err)
	}
	defer func() {
		_ = lock.Release()
	}()

	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	clients, err := loadClients(ctx, cfg.Provider.Region)
	if err != nil {
		return nil, appErrs.Wrap(appErrs.CodeRemoveFailed, "load aws config failed", err)
	}

	if err := recoverIfNeeded(ctx, root, cfg.Service, stage, clients); err != nil {
		return nil, appErrs.Wrap(appErrs.CodeRemoveFailed, "recovery check failed", err)
	}

	apiName := httpAPIName(cfg, stage)

	apisOut, err := clients.APIGW.GetApis(ctx, nil)
	if err != nil {
		return nil, appErrs.Wrap(appErrs.CodeRemoveFailed, "list apis failed", err)
	}

	for _, api := range apisOut.Items {
		if api.Name != nil && *api.Name == apiName && api.ApiId != nil {
			_, err := clients.APIGW.DeleteApi(ctx, &apigatewayv2.DeleteApiInput{
				ApiId: api.ApiId,
			})
			if err != nil {
				return nil, appErrs.Wrap(appErrs.CodeRemoveFailed, "delete api failed", err)
			}
			break
		}
	}

	for fnName := range cfg.Functions {
		name := functionName(cfg, stage, fnName)

		if err := deleteFunctionURL(ctx, clients, name); err != nil {
			return nil, appErrs.Wrap(appErrs.CodeRemoveFailed, "delete function url failed: "+fnName, err)
		}

		_, err := clients.Lambda.DeleteFunction(ctx, &lambda.DeleteFunctionInput{
			FunctionName: &name,
		})
		if err != nil && !isLambdaNotFound(err) {
			return nil, appErrs.Wrap(appErrs.CodeRemoveFailed, "delete lambda failed: "+fnName, err)
		}

		if err == nil {
			if err := waitUntilFunctionDeleted(ctx, clients, name); err != nil {
				return nil, appErrs.Wrap(appErrs.CodeRemoveFailed, "wait lambda delete failed: "+fnName, err)
			}
		}

		if err := deleteLambdaExecutionRole(ctx, clients, cfg, stage, fnName); err != nil {
			return nil, appErrs.Wrap(appErrs.CodeRemoveFailed, "delete lambda role failed: "+fnName, err)
		}
	}

	if stepFunctions, err := stepFunctionsFromConfig(cfg, root); err == nil {
		for _, decl := range stepFunctions {
			listOut, listErr := clients.SFN.ListStateMachines(ctx, &sfnv2.ListStateMachinesInput{})
			if listErr != nil {
				return nil, appErrs.Wrap(appErrs.CodeRemoveFailed, "list step functions failed", listErr)
			}
			for _, sm := range listOut.StateMachines {
				if sm.Name == nil || sm.StateMachineArn == nil {
					continue
				}
				if *sm.Name != decl.Name {
					continue
				}
				_, delErr := clients.SFN.DeleteStateMachine(ctx, &sfnv2.DeleteStateMachineInput{StateMachineArn: sm.StateMachineArn})
				if delErr != nil {
					return nil, appErrs.Wrap(appErrs.CodeRemoveFailed, "delete step function failed: "+decl.Name, delErr)
				}
			}
		}
	} else {
		return nil, appErrs.Wrap(appErrs.CodeRemoveFailed, "parse step functions config failed", err)
	}

	buildDir := filepath.Join(root, ".runfabric", "build")
	if err := os.RemoveAll(buildDir); err != nil {
		return nil, fmt.Errorf("remove local build dir: %w", err)
	}

	return &providers.RemoveResult{
		Provider: p.Name(),
		Removed:  true,
	}, nil
}
