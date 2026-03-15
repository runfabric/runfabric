// Package resources documents DigitalOcean App Platform resources and triggers
// per the Trigger Capability Matrix (internal/planner/capability_matrix.go).
//
// Actions (segregated): deploy (deploy.go), remove (remove.go), invoke (invoke.go), logs (logs.go).
// Supported triggers: http, cron only. Resources created by deploy: App (functions + jobs).
package resources
