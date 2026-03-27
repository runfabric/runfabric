package alibaba

import (
	"context"
	"fmt"
	"strings"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// EnsureHTTP ensures HTTP trigger for the function (FC trigger type "http").
// Per Trigger Capability Matrix: alibaba-fc supports http.
func EnsureHTTP(ctx context.Context, cfg sdkprovider.Config, stage, functionName string) error {
	region := sdkprovider.ProviderRegion(cfg)
	client, err := fcClientFromEnv(region)
	if err != nil {
		return err
	}
	service := sdkprovider.Service(cfg)
	serviceName := service + "-" + stage
	funcName := fmt.Sprintf("%s-%s-%s", service, stage, functionName)
	_, err = client.CreateTrigger(ctx, serviceName, funcName, "http", "http", map[string]any{
		"authType": "anonymous",
		"methods":  []string{"GET", "POST", "PUT", "DELETE"},
	})
	if err != nil && !strings.Contains(err.Error(), "TriggerAlreadyExists") && !strings.Contains(err.Error(), "already exist") {
		return err
	}
	return nil
}

// EnsureCron ensures timer (cron) trigger. Per capability matrix: alibaba-fc supports cron.
func EnsureCron(ctx context.Context, cfg sdkprovider.Config, stage, functionName, expression string) error {
	if expression == "" {
		return nil
	}
	region := sdkprovider.ProviderRegion(cfg)
	client, err := fcClientFromEnv(region)
	if err != nil {
		return err
	}
	service := sdkprovider.Service(cfg)
	serviceName := service + "-" + stage
	funcName := fmt.Sprintf("%s-%s-%s", service, stage, functionName)
	triggerName := "timer-" + functionName
	_, err = client.CreateTrigger(ctx, serviceName, funcName, triggerName, "timer", map[string]any{
		"cronExpression": expression,
		"enable":         true,
	})
	if err != nil && !strings.Contains(err.Error(), "TriggerAlreadyExists") && !strings.Contains(err.Error(), "already exist") {
		return err
	}
	return nil
}

// EnsureQueue ensures MNS queue trigger. Per capability matrix: alibaba-fc supports queue.
func EnsureQueue(ctx context.Context, cfg sdkprovider.Config, stage, functionName, queue string) error {
	if queue == "" {
		return nil
	}
	region := sdkprovider.ProviderRegion(cfg)
	client, err := fcClientFromEnv(region)
	if err != nil {
		return err
	}
	service := sdkprovider.Service(cfg)
	serviceName := service + "-" + stage
	funcName := fmt.Sprintf("%s-%s-%s", service, stage, functionName)
	triggerName := "mns-" + functionName
	_, err = client.CreateTrigger(ctx, serviceName, funcName, triggerName, "mns_topic", map[string]any{
		"topicName": queue,
		"region":    sdkprovider.Env("ALIBABA_FC_REGION"),
	})
	if err != nil && !strings.Contains(err.Error(), "TriggerAlreadyExists") && !strings.Contains(err.Error(), "already exist") {
		return err
	}
	return nil
}

// EnsureStorage ensures OSS trigger. Per capability matrix: alibaba-fc supports storage.
func EnsureStorage(ctx context.Context, cfg sdkprovider.Config, stage, functionName, bucket, prefix string) error {
	if bucket == "" {
		return nil
	}
	region := sdkprovider.ProviderRegion(cfg)
	client, err := fcClientFromEnv(region)
	if err != nil {
		return err
	}
	service := sdkprovider.Service(cfg)
	serviceName := service + "-" + stage
	funcName := fmt.Sprintf("%s-%s-%s", service, stage, functionName)
	triggerName := "oss-" + functionName
	cfgMap := map[string]any{
		"events": []string{"oss:ObjectCreated:*"},
		"bucket": bucket,
	}
	if prefix != "" {
		cfgMap["prefix"] = prefix
	}
	_, err = client.CreateTrigger(ctx, serviceName, funcName, triggerName, "oss", cfgMap)
	if err != nil && !strings.Contains(err.Error(), "TriggerAlreadyExists") && !strings.Contains(err.Error(), "already exist") {
		return err
	}
	return nil
}

func fcClientFromEnv(region string) (*fcClient, error) {
	accessKey := sdkprovider.Env("ALIBABA_ACCESS_KEY_ID")
	secretKey := sdkprovider.Env("ALIBABA_ACCESS_KEY_SECRET")
	if accessKey == "" || secretKey == "" {
		return nil, fmt.Errorf("ALIBABA_ACCESS_KEY_ID and ALIBABA_ACCESS_KEY_SECRET are required")
	}
	accountID := sdkprovider.Env("ALIBABA_FC_ACCOUNT_ID")
	if accountID == "" {
		return nil, fmt.Errorf("ALIBABA_FC_ACCOUNT_ID is required")
	}
	if region == "" {
		region = sdkprovider.Env("ALIBABA_FC_REGION")
	}
	if region == "" {
		region = "cn-hangzhou"
	}
	return newFCClient(accountID, region, accessKey, secretKey), nil
}
