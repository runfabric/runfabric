# Provider Reference

This document is the provider capability matrix for the current RunFabric engine.

## Provider Matrix

| Provider               | Compute Deploy/Invoke  | Doctor/Plan | Native Orchestration | AI Step Execution      |
| ---------------------- | ---------------------- | ----------- | -------------------- | ---------------------- |
| AWS Lambda             | Yes                    | Yes         | Step Functions       | No (core runtime only) |
| GCP Functions          | Yes                    | Yes         | Cloud Workflows      | No (core runtime only) |
| Azure Functions        | Yes                    | Yes         | Durable Functions    | No (core runtime only) |
| Alibaba FC             | Yes                    | Yes         | No                   | No                     |
| Cloudflare Workers     | Yes                    | Yes         | No                   | No                     |
| DigitalOcean Functions | Yes                    | Yes         | No                   | No                     |
| Fly Machines           | Yes                    | Yes         | No                   | No                     |
| IBM OpenWhisk          | Yes                    | Yes         | No                   | No                     |
| Kubernetes             | Yes                    | Yes         | No                   | No                     |
| Linode                 | Yes                    | Yes         | No                   | No                     |
| Netlify                | Yes                    | Yes         | No                   | No                     |
| Vercel                 | Yes                    | Yes         | No                   | No                     |
| DevStream              | Routing/tunnel support | Yes         | No                   | No                     |

## Important Boundary

- Provider adapters own deploy/invoke/logs/plan/doctor behavior.
- AI workflow execution (`ai-retrieval`, `ai-generate`, `ai-structured`, `ai-eval`) is executed by `platform/deploy/controlplane`.
- Providers do not import or execute `workflow_ai_runtime` logic.

## References

- Provider + AI analysis: [PROVIDER_AI_WORKFLOW_ANALYSIS.md](../PROVIDER_AI_WORKFLOW_ANALYSIS.md)
- Provider setup and credentials: [PROVIDER_SETUP.md](PROVIDER_SETUP.md)
- Deploy provider notes: [DEPLOY_PROVIDERS.md](DEPLOY_PROVIDERS.md)
