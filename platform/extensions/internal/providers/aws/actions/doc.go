// Package actions documents the AWS provider lifecycle actions (deploy, remove, invoke, logs).
//
// Entry points are on the Provider in the parent aws package:
//   - Deploy: deploy.go (redirects to control plane); implementation in deploy_resume.go, deploy_engine.go, deploy_plan.go
//   - Remove: remove.go
//   - Invoke: invoke.go
//   - Logs: logs.go
//
// Build and plan: build.go, plan.go, deploy_plan.go. Recovery: recovery.go, rollback.go.
package actions
