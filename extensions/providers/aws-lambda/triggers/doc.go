// Package triggers implements AWS trigger resources per the Trigger Capability Matrix (aws-lambda).
//
// Supported triggers:
//   - cron: EnsureCronRules (CloudWatch Events / EventBridge) — cron.go
//   - queue: EnsureQueueTriggers (SQS event source mappings) — queue.go
//   - storage: EnsureStorageTriggers (S3 bucket notifications) — storage.go
//   - eventbridge: EnsureEventBridgeRules (EventBridge rules and targets) — eventbridge.go
//
// HTTP is handled by API Gateway and function URL in the parent package (apigw_*.go, function_url.go).
package triggers
