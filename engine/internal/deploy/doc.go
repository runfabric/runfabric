// Package deploy provides the engine's deployment paths.
//
//   - api: API-based deploy/remove/invoke/logs (internal/deploy/api). Used by app for non-AWS providers.
//     No provider CLIs required; uses provider REST APIs and SDKs.
//   - cli: CLI-based deploy (internal/deploy/cli). Optional path using wrangler, vercel, fly, gcloud, etc.
package deploy
