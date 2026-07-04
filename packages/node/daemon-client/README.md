# @runfabric/daemon-client

Node HTTP client for the RunFabric daemon (`runfabricd`). This is the official
client for the config API served by `platform/daemon/configapi` — use it
instead of hand-rolling `fetch` calls so the wire contract (query params,
auth, per-request credentials, error shape) lives in one place.

```js
const { DaemonClient } = require('@runfabric/daemon-client');

const client = new DaemonClient({
  baseUrl: process.env.RUNFABRIC_DAEMON_URL,
  apiKey: process.env.RUNFABRIC_API_KEY,
});

const result = await client.deploy({
  configPath: 'tenant/project/dev/runfabric.yml', // relative to the daemon's workspace root
  stage: 'dev',
  providerHeaders: { 'X-Provider-Aws-Access-Key-Id': '…', /* … */ },
});
if (result.ok) {
  console.log(result.data.deploymentId, result.data.outputs);
} else {
  console.error(result.error);
}
```

## Wire contract

- `POST /deploy | /remove | /plan | /validate | /resolve | /releases ?config=<rel>&stage=<stage>`
  — no request body; the daemon operates on the `runfabric.yml` + built
  artifacts that live on **its** filesystem, addressed by the `config` path
  relative to its workspace root.
- Auth: `X-API-Key` header (a non-loopback daemon refuses to start without one).
- Per-request provider credentials: `X-Provider-*` headers. Each provider
  declares its accepted headers in its `CredentialVars`
  (`extensions/providers/<id>/credentials.go`) and `plugin.yaml`
  (`credentials:`); the daemon applies them to the process env for that one
  operation and restores the ambient env afterwards.
- Success: `/deploy`, `/plan`, `/remove`, `/resolve`, `/releases` return the
  engine payload verbatim; `/validate` returns `{ok:true}`.
- Errors: `{ok:false, error}` with a 4xx status.

## API

Every method takes `{ configPath?, stage?, providerHeaders?, timeoutMs? }` and
resolves to `{ ok: true, status, data }` or `{ ok: false, status?, error }` —
HTTP and network failures never throw.

| Method       | Default timeout |
| ------------ | --------------- |
| `deploy`     | 300 s           |
| `remove`     | 120 s           |
| `plan`       | 60 s            |
| `validate`   | 60 s            |
| `resolve`    | 60 s            |
| `releases`   | 30 s            |

Helpers: `awsProviderHeaders({accessKeyId, secretAccessKey, sessionToken?, region?})`
builds the AWS header set; `DEFAULT_TIMEOUTS_MS` exposes the defaults.

Requires Node ≥ 18 (global `fetch`); pass a custom `fetch` in options to
override.
