package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/runfabric/runfabric/internal/config"
)

func ensureLambdaExecutionRole(ctx context.Context, clients *AWSClients, cfg *config.Config, stage, fn string) (string, bool, error) {
	name := roleName(cfg, stage, fn)

	getOut, err := clients.IAM.GetRole(ctx, &iam.GetRoleInput{
		RoleName: &name,
	})
	if err == nil && getOut.Role != nil && getOut.Role.Arn != nil {
		return *getOut.Role.Arn, false, nil
	}

	if err != nil && !isIAMNoSuchEntity(err) {
		return "", false, fmt.Errorf("get role: %w", err)
	}

	trustPolicy := map[string]any{
		"Version": "2012-10-17",
		"Statement": []map[string]any{
			{
				"Effect": "Allow",
				"Principal": map[string]any{
					"Service": "lambda.amazonaws.com",
				},
				"Action": "sts:AssumeRole",
			},
		},
	}

	policyJSON, err := json.Marshal(trustPolicy)
	if err != nil {
		return "", false, fmt.Errorf("marshal trust policy: %w", err)
	}

	createOut, err := clients.IAM.CreateRole(ctx, &iam.CreateRoleInput{
		RoleName:                 &name,
		AssumeRolePolicyDocument: str(string(policyJSON)),
	})
	if err != nil {
		return "", false, fmt.Errorf("create role: %w", err)
	}

	_, err = clients.IAM.AttachRolePolicy(ctx, &iam.AttachRolePolicyInput{
		RoleName:  &name,
		PolicyArn: str("arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"),
	})
	if err != nil {
		return "", false, fmt.Errorf("attach role policy: %w", err)
	}

	time.Sleep(10 * time.Second)

	if createOut.Role == nil || createOut.Role.Arn == nil {
		return "", false, fmt.Errorf("created role missing arn")
	}

	return *createOut.Role.Arn, true, nil
}

func deleteLambdaExecutionRole(ctx context.Context, clients *AWSClients, cfg *config.Config, stage, fn string) error {
	name := roleName(cfg, stage, fn)

	_, err := clients.IAM.GetRole(ctx, &iam.GetRoleInput{
		RoleName: &name,
	})
	if err != nil {
		if isIAMNoSuchEntity(err) {
			return nil
		}
		return fmt.Errorf("get role before delete: %w", err)
	}

	attached, err := clients.IAM.ListAttachedRolePolicies(ctx, &iam.ListAttachedRolePoliciesInput{
		RoleName: &name,
	})
	if err != nil {
		return fmt.Errorf("list attached policies: %w", err)
	}

	for _, p := range attached.AttachedPolicies {
		if p.PolicyArn == nil {
			continue
		}
		_, err := clients.IAM.DetachRolePolicy(ctx, &iam.DetachRolePolicyInput{
			RoleName:  &name,
			PolicyArn: p.PolicyArn,
		})
		if err != nil {
			return fmt.Errorf("detach role policy: %w", err)
		}
	}

	_, err = clients.IAM.DeleteRole(ctx, &iam.DeleteRoleInput{
		RoleName: &name,
	})
	if err != nil {
		return fmt.Errorf("delete role: %w", err)
	}

	return nil
}
