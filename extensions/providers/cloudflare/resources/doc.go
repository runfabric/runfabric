// Package resources documents Cloudflare Workers resources and triggers
// per the Trigger Capability Matrix (internal/planner/capability_matrix.go).
//
// Actions: deploy (api_deploy.go), remove (api_remove.go), invoke (api_invoke.go), logs (api_logs.go).
// Supported triggers: http, cron only. Resources: Worker script.
package resources
