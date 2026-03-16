# Examples and Trigger Capability Matrix

This document describes which **triggers** each provider supports. The Go engine enforces this matrix in `internal/planner` (see [ARCHITECTURE.md](ARCHITECTURE.md)).

## Trigger capability matrix

| Provider              | http | cron | queue | storage | eventbridge | pubsub |
|-----------------------|------|------|-------|---------|-------------|--------|
| aws-lambda            | ✓    | ✓    | ✓     | ✓       | ✓           | —      |
| gcp-functions         | ✓    | ✓    | ✓     | ✓       | —           | ✓      |
| azure-functions       | ✓    | ✓    | ✓     | ✓       | —           | —      |
| kubernetes            | ✓    | ✓    | —     | —       | —           | —      |
| cloudflare-workers    | ✓    | ✓    | —     | —       | —           | —      |
| vercel                | ✓    | ✓    | —     | —       | —           | —      |
| netlify               | ✓    | ✓    | —     | —       | —           | —      |
| alibaba-fc            | ✓    | ✓    | ✓     | ✓       | —           | —      |
| digitalocean-functions| ✓    | ✓    | —     | —       | —           | —      |
| fly-machines          | ✓    | —    | —     | —       | —           | —      |
| ibm-openwhisk         | ✓    | ✓    | —     | —       | —           | —      |

Use **`runfabric init`** with `--template` and `--provider` to scaffold a project; the CLI validates the combination against this matrix.

**See also:** [COMMAND_REFERENCE.md](COMMAND_REFERENCE.md), [PROVIDER_SETUP.md](PROVIDER_SETUP.md).
