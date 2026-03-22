// Package resources holds AWS resource types and helpers aligned with the Trigger Capability Matrix.
//
// Resource types and API helpers:
//   - lambda.go: LambdaFunction and Lambda resource helpers
//   - apigw.go: API Gateway (HTTP API) resources
//   - iam.go: IAM role and policy helpers
//   - s3.go: S3 bucket notification (storage trigger)
//   - sqs.go: SQS event source (queue trigger)
//   - eventbridge.go: EventBridge rules (cron, eventbridge trigger)
//
// Supported triggers (internal/planner/capability_matrix.go "aws-lambda"):
// http, cron, queue, storage, eventbridge. See triggers/ for EnsureCronRules, EnsureQueueTriggers, etc.
package resources
