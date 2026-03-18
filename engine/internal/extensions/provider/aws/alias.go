package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/runfabric/runfabric/engine/internal/config"

	"github.com/aws/aws-sdk-go-v2/service/lambda"
)

const defaultAliasName = "live"

// publishVersionAndEnsureAlias publishes a new Lambda version and ensures the alias points to it.
// For blue-green: publish then switch alias. For canary: optional delay (CanaryIntervalMinutes) then switch.
// Returns the alias ARN so API Gateway can invoke the function via the alias.
func publishVersionAndEnsureAlias(
	ctx context.Context,
	clients *AWSClients,
	functionName string,
	cfg *config.Config,
) (aliasARN string, err error) {
	pub, err := clients.Lambda.PublishVersion(ctx, &lambda.PublishVersionInput{
		FunctionName: &functionName,
	})
	if err != nil {
		return "", fmt.Errorf("publish version: %w", err)
	}
	if pub.Version == nil || *pub.Version == "" {
		return "", fmt.Errorf("publish version: missing version in response")
	}
	version := *pub.Version

	// Canary: wait before switching traffic (optional delay)
	if cfg.Deploy != nil && cfg.Deploy.Strategy == "canary" && cfg.Deploy.CanaryIntervalMinutes > 0 {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(time.Duration(cfg.Deploy.CanaryIntervalMinutes) * time.Minute):
			// proceed to update alias
		}
	}

	// Ensure alias "live" exists and points to this version
	aliasName := defaultAliasName
	getAlias, err := clients.Lambda.GetAlias(ctx, &lambda.GetAliasInput{
		FunctionName: &functionName,
		Name:         &aliasName,
	})
	if err != nil {
		if !isLambdaNotFound(err) {
			return "", fmt.Errorf("get alias: %w", err)
		}
		// Alias does not exist: create it
		createOut, createErr := clients.Lambda.CreateAlias(ctx, &lambda.CreateAliasInput{
			FunctionName:    &functionName,
			FunctionVersion: &version,
			Name:            &aliasName,
			Description:     str("runfabric blue-green/canary target"),
		})
		if createErr != nil {
			return "", fmt.Errorf("create alias %s: %w", aliasName, createErr)
		}
		if createOut.AliasArn != nil {
			return *createOut.AliasArn, nil
		}
		return "", fmt.Errorf("create alias: missing alias ARN")
	}
	// Alias exists: update to new version
	upd, err := clients.Lambda.UpdateAlias(ctx, &lambda.UpdateAliasInput{
		FunctionName:    &functionName,
		Name:            &aliasName,
		FunctionVersion: &version,
	})
	if err != nil {
		return "", fmt.Errorf("update alias %s: %w", aliasName, err)
	}
	if upd.AliasArn != nil {
		return *upd.AliasArn, nil
	}
	if getAlias.AliasArn != nil {
		return *getAlias.AliasArn, nil
	}
	return "", fmt.Errorf("alias ARN unknown")
}
