package alibaba

import (
	"context"
	"fmt"
	"strings"

	"github.com/runfabric/runfabric/platform/core/model/config"
	planner "github.com/runfabric/runfabric/platform/core/planner/engine"
	"github.com/runfabric/runfabric/platform/deploy/apiutil"
)

// EnsureHTTP ensures HTTP trigger for the function (FC trigger type "http").
// Per Trigger Capability Matrix: alibaba-fc supports http.
func EnsureHTTP(ctx context.Context, cfg *config.Config, stage, functionName string) error {
	if !planner.SupportsTrigger("alibaba-fc", planner.TriggerHTTP) {
		return fmt.Errorf("alibaba-fc does not support http trigger")
	}
	client, err := fcClientFromEnv(cfg, stage)
	if err != nil {
		return err
	}
	serviceName := cfg.Service + "-" + stage
	funcName := fmt.Sprintf("%s-%s-%s", cfg.Service, stage, functionName)
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
func EnsureCron(ctx context.Context, cfg *config.Config, stage, functionName, expression string) error {
	if !planner.SupportsTrigger("alibaba-fc", planner.TriggerCron) {
		return fmt.Errorf("alibaba-fc does not support cron trigger")
	}
	if expression == "" {
		return nil
	}
	client, err := fcClientFromEnv(cfg, stage)
	if err != nil {
		return err
	}
	serviceName := cfg.Service + "-" + stage
	funcName := fmt.Sprintf("%s-%s-%s", cfg.Service, stage, functionName)
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
func EnsureQueue(ctx context.Context, cfg *config.Config, stage, functionName, queue string) error {
	if !planner.SupportsTrigger("alibaba-fc", planner.TriggerQueue) {
		return fmt.Errorf("alibaba-fc does not support queue trigger")
	}
	if queue == "" {
		return nil
	}
	client, err := fcClientFromEnv(cfg, stage)
	if err != nil {
		return err
	}
	serviceName := cfg.Service + "-" + stage
	funcName := fmt.Sprintf("%s-%s-%s", cfg.Service, stage, functionName)
	triggerName := "mns-" + functionName
	_, err = client.CreateTrigger(ctx, serviceName, funcName, triggerName, "mns_topic", map[string]any{
		"topicName": queue,
		"region":    apiutil.Env("ALIBABA_FC_REGION"),
	})
	if err != nil && !strings.Contains(err.Error(), "TriggerAlreadyExists") && !strings.Contains(err.Error(), "already exist") {
		return err
	}
	return nil
}

// EnsureStorage ensures OSS trigger. Per capability matrix: alibaba-fc supports storage.
func EnsureStorage(ctx context.Context, cfg *config.Config, stage, functionName, bucket, prefix string) error {
	if !planner.SupportsTrigger("alibaba-fc", planner.TriggerStorage) {
		return fmt.Errorf("alibaba-fc does not support storage trigger")
	}
	if bucket == "" {
		return nil
	}
	client, err := fcClientFromEnv(cfg, stage)
	if err != nil {
		return err
	}
	serviceName := cfg.Service + "-" + stage
	funcName := fmt.Sprintf("%s-%s-%s", cfg.Service, stage, functionName)
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

func fcClientFromEnv(cfg *config.Config, stage string) (*fcClient, error) {
	accessKey := apiutil.Env("ALIBABA_ACCESS_KEY_ID")
	secretKey := apiutil.Env("ALIBABA_ACCESS_KEY_SECRET")
	if accessKey == "" || secretKey == "" {
		return nil, fmt.Errorf("ALIBABA_ACCESS_KEY_ID and ALIBABA_ACCESS_KEY_SECRET are required")
	}
	accountID := apiutil.Env("ALIBABA_FC_ACCOUNT_ID")
	if accountID == "" {
		return nil, fmt.Errorf("ALIBABA_FC_ACCOUNT_ID is required")
	}
	region := cfg.Provider.Region
	if region == "" {
		region = apiutil.Env("ALIBABA_FC_REGION")
	}
	if region == "" {
		region = "cn-hangzhou"
	}
	return newFCClient(accountID, region, accessKey, secretKey), nil
}
