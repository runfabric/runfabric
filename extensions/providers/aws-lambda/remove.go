package aws

import (
	"context"
	"fmt"
	"strings"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	cloudwatchlogsv2 "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	iamv2 "github.com/aws/aws-sdk-go-v2/service/iam"
	lambdav2 "github.com/aws/aws-sdk-go-v2/service/lambda"
	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// Remove tears down everything Deploy created for a stage: each function's URL
// config, the function itself, and its CloudWatch log group, plus the shared
// execution role (runfabric-<service>-<stage>-exec). Already-absent resources
// are ignored so remove is idempotent.
func (p *Provider) Remove(ctx context.Context, req sdkprovider.RemoveRequest) (*sdkprovider.RemoveResult, error) {
	cfg := req.Config
	stage := req.Stage
	service := sdkprovider.Service(cfg)
	region := sdkprovider.ProviderRegion(cfg)
	if region == "" {
		region = sdkprovider.Env("AWS_REGION")
	}

	clients, err := loadClients(ctx, region)
	if err != nil {
		return nil, fmt.Errorf("load aws clients: %w", err)
	}

	var problems []string
	for fn := range sdkprovider.Functions(cfg) {
		functionName := fmt.Sprintf("%s-%s-%s", service, stage, fn)

		// Function URL config (best-effort; present only for http triggers).
		if err := deleteFunctionURL(ctx, clients, functionName); err != nil && !isLambdaNotFound(err) {
			problems = append(problems, fmt.Sprintf("delete function url %s: %v", functionName, err))
		}
		// The function itself.
		if _, err := clients.Lambda.DeleteFunction(ctx, &lambdav2.DeleteFunctionInput{
			FunctionName: awssdk.String(functionName),
		}); err != nil && !isLambdaNotFound(err) {
			problems = append(problems, fmt.Sprintf("delete function %s: %v", functionName, err))
		}
		// Its log group (auto-created on first invoke; best-effort).
		logGroup := "/aws/lambda/" + functionName
		if _, err := clients.Logs.DeleteLogGroup(ctx, &cloudwatchlogsv2.DeleteLogGroupInput{
			LogGroupName: awssdk.String(logGroup),
		}); err != nil && !isLogsNotFound(err) {
			problems = append(problems, fmt.Sprintf("delete log group %s: %v", logGroup, err))
		}
	}

	// Shared execution role — detach/delete its policies, then the role.
	roleName := fmt.Sprintf("runfabric-%s-%s-exec", service, stage)
	if err := deleteExecRole(ctx, clients, roleName); err != nil {
		problems = append(problems, fmt.Sprintf("delete role %s: %v", roleName, err))
	}

	if len(problems) > 0 {
		return nil, fmt.Errorf("remove incomplete: %s", strings.Join(problems, "; "))
	}
	return &sdkprovider.RemoveResult{
		Provider: p.Name(),
		Removed:  true,
	}, nil
}

// deleteExecRole detaches managed policies and removes inline policies from the
// role before deleting it (IAM requires an empty role). A missing role is a
// no-op so remove is idempotent.
func deleteExecRole(ctx context.Context, clients *AWSClients, roleName string) error {
	attached, err := clients.IAM.ListAttachedRolePolicies(ctx, &iamv2.ListAttachedRolePoliciesInput{
		RoleName: awssdk.String(roleName),
	})
	if err != nil {
		if isIAMNoSuchEntity(err) {
			return nil
		}
		return fmt.Errorf("list attached policies: %w", err)
	}
	for _, pol := range attached.AttachedPolicies {
		if _, err := clients.IAM.DetachRolePolicy(ctx, &iamv2.DetachRolePolicyInput{
			RoleName:  awssdk.String(roleName),
			PolicyArn: pol.PolicyArn,
		}); err != nil && !isIAMNoSuchEntity(err) {
			return fmt.Errorf("detach %s: %w", awssdk.ToString(pol.PolicyArn), err)
		}
	}

	inline, err := clients.IAM.ListRolePolicies(ctx, &iamv2.ListRolePoliciesInput{
		RoleName: awssdk.String(roleName),
	})
	if err != nil && !isIAMNoSuchEntity(err) {
		return fmt.Errorf("list inline policies: %w", err)
	}
	if inline != nil {
		for _, name := range inline.PolicyNames {
			if _, err := clients.IAM.DeleteRolePolicy(ctx, &iamv2.DeleteRolePolicyInput{
				RoleName:   awssdk.String(roleName),
				PolicyName: awssdk.String(name),
			}); err != nil && !isIAMNoSuchEntity(err) {
				return fmt.Errorf("delete inline policy %s: %w", name, err)
			}
		}
	}

	if _, err := clients.IAM.DeleteRole(ctx, &iamv2.DeleteRoleInput{
		RoleName: awssdk.String(roleName),
	}); err != nil && !isIAMNoSuchEntity(err) {
		return fmt.Errorf("delete role: %w", err)
	}
	return nil
}
