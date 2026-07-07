'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const http = require('node:http');

const { DaemonClient, awsProviderHeaders, DEFAULT_TIMEOUTS_MS } = require('./index.js');

/** Start a stub daemon that records requests and replies per route. */
function startStub(routes) {
  const seen = [];
  const server = http.createServer((req, res) => {
    const url = new URL(req.url, 'http://localhost');
    const chunks = [];
    req.on('data', (c) => chunks.push(c));
    req.on('end', () => {
      const raw = Buffer.concat(chunks).toString('utf8');
      let json;
      try {
        json = raw ? JSON.parse(raw) : undefined;
      } catch {
        json = undefined;
      }
      seen.push({
        method: req.method,
        path: url.pathname,
        query: Object.fromEntries(url.searchParams),
        headers: req.headers,
        body: json,
        rawBody: raw,
      });
      const route = routes[url.pathname] || { status: 404, body: { ok: false, error: 'not found' } };
      res.writeHead(route.status, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify(route.body));
    });
  });
  return new Promise((resolve) => {
    server.listen(0, '127.0.0.1', () => {
      resolve({
        baseUrl: `http://127.0.0.1:${server.address().port}`,
        seen,
        close: () => new Promise((r) => server.close(r)),
      });
    });
  });
}

test('deploy sends config/stage query, API key, and provider headers; returns raw payload', async () => {
  const stub = await startStub({
    '/deploy': { status: 200, body: { provider: 'aws-lambda', deploymentId: 'dpl-1', outputs: { url: 'x' } } },
  });
  try {
    const client = new DaemonClient({ baseUrl: stub.baseUrl + '/', apiKey: 'secret' });
    const result = await client.deploy({
      configPath: 't1/p1/dev/runfabric.yml',
      stage: 'dev',
      providerHeaders: { 'X-Provider-Aws-Access-Key-Id': 'AKIA', 'X-Provider-Empty': '' },
    });
    assert.equal(result.ok, true);
    assert.equal(result.data.deploymentId, 'dpl-1');
    const req = stub.seen[0];
    assert.equal(req.method, 'POST');
    assert.equal(req.path, '/deploy');
    assert.deepEqual(req.query, { config: 't1/p1/dev/runfabric.yml', stage: 'dev' });
    assert.equal(req.headers['x-api-key'], 'secret');
    assert.equal(req.headers['x-provider-aws-access-key-id'], 'AKIA');
    assert.equal('x-provider-empty' in req.headers, false, 'empty header values are dropped');
  } finally {
    await stub.close();
  }
});

test('error responses map to ok:false with the daemon error message', async () => {
  const stub = await startStub({
    '/validate': { status: 400, body: { ok: false, error: 'field nonsense not found' } },
  });
  try {
    const client = new DaemonClient({ baseUrl: stub.baseUrl });
    const result = await client.validate({ configPath: 'runfabric.yml' });
    assert.equal(result.ok, false);
    assert.equal(result.status, 400);
    assert.match(result.error, /nonsense/);
  } finally {
    await stub.close();
  }
});

test('validate success returns {ok:true} payload; defaults config to runfabric.yml', async () => {
  const stub = await startStub({ '/validate': { status: 200, body: { ok: true } } });
  try {
    const client = new DaemonClient({ baseUrl: stub.baseUrl });
    const result = await client.validate();
    assert.equal(result.ok, true);
    assert.deepEqual(result.data, { ok: true });
    assert.deepEqual(stub.seen[0].query, { config: 'runfabric.yml' });
  } finally {
    await stub.close();
  }
});

test('network failure returns ok:false instead of throwing', async () => {
  const client = new DaemonClient({ baseUrl: 'http://127.0.0.1:1', timeoutsMs: { plan: 500 } });
  const result = await client.plan({ stage: 'dev' });
  assert.equal(result.ok, false);
  assert.ok(result.error.length > 0);
});

test('unset base URL: available() is false and calls fail cleanly', async () => {
  const client = new DaemonClient({ baseUrl: '' });
  assert.equal(client.available(), false);
  const result = await client.deploy({});
  assert.equal(result.ok, false);
  assert.match(result.error, /base URL/);
});

test('awsProviderHeaders builds the legacy AWS header set', () => {
  assert.deepEqual(
    awsProviderHeaders({ accessKeyId: 'A', secretAccessKey: 'S', sessionToken: 'T', region: 'us-east-1' }),
    {
      'X-Provider-Aws-Access-Key-Id': 'A',
      'X-Provider-Aws-Secret-Access-Key': 'S',
      'X-Provider-Aws-Session-Token': 'T',
      'X-Provider-Aws-Region': 'us-east-1',
    },
  );
  assert.deepEqual(awsProviderHeaders(undefined), {});
  assert.deepEqual(awsProviderHeaders({ accessKeyId: 'A' }), {});
});

test('per-call timeout override wins over defaults', async () => {
  assert.equal(DEFAULT_TIMEOUTS_MS.deploy, 300_000);
  let sawSignal = null;
  const client = new DaemonClient({
    baseUrl: 'http://example.invalid',
    fetch: async (_url, init) => {
      sawSignal = init.signal;
      return { ok: true, status: 200, json: async () => ({}) };
    },
  });
  const result = await client.deploy({ timeoutMs: 1234 });
  assert.equal(result.ok, true);
  assert.ok(sawSignal instanceof AbortSignal);
});

test('CREDENTIALS contract ships provider and state declarations', () => {
  const { CREDENTIALS } = require('./index.js');
  const aws = CREDENTIALS.providers['aws-lambda'];
  assert.ok(Array.isArray(aws) && aws.length >= 4);
  assert.ok(aws.some((c) => c.envKey === 'AWS_ACCESS_KEY_ID' && c.header === 'X-Provider-Aws-Access-Key-Id'));
  const pg = CREDENTIALS.state.postgres;
  assert.ok(pg.some((c) => c.envKey === 'RUNFABRIC_STATE_POSTGRES_URL' && c.header === 'X-State-Postgres-Url'));
});

test('traceparent, tracestate and request id are sent; correlation ids come back', async () => {
  const { newTraceparent, traceIdOf } = require('./index.js');
  const tp = newTraceparent();
  assert.match(tp, /^00-[0-9a-f]{32}-[0-9a-f]{16}-01$/);
  const traceId = traceIdOf(tp);
  assert.equal(traceId.length, 32);
  assert.equal(traceIdOf('garbage'), '');

  const stub = await startStub({ '/deploy': { status: 200, body: { deploymentId: 'dpl-2' } } });
  try {
    const client = new DaemonClient({ baseUrl: stub.baseUrl });
    const result = await client.deploy({ traceparent: tp, tracestate: 'rf=1', requestId: 'req-7' });
    assert.equal(result.ok, true);
    const req = stub.seen[0];
    assert.equal(req.headers['traceparent'], tp);
    assert.equal(req.headers['tracestate'], 'rf=1');
    assert.equal(req.headers['x-request-id'], 'req-7');
    // Stub does not echo X-Trace-Id; the client falls back to the traceparent's id.
    assert.equal(result.traceId, traceId);
    assert.equal(result.requestId, 'req-7');
  } finally {
    await stub.close();
  }
});

test('daemon-echoed X-Trace-Id wins over the traceparent fallback', async () => {
  const echoed = 'a'.repeat(32);
  const seen = [];
  const client = new DaemonClient({
    baseUrl: 'http://example.invalid',
    fetch: async (_url, init) => {
      seen.push(init.headers);
      return {
        ok: true,
        status: 200,
        headers: new Map([['x-trace-id', echoed], ['x-request-id', 'gen-1']]),
        json: async () => ({}),
      };
    },
  });
  const result = await client.deploy({ traceparent: '00-' + 'b'.repeat(32) + '-' + 'c'.repeat(16) + '-01' });
  assert.equal(result.ok, true);
  assert.equal(result.traceId, echoed);
  assert.equal(result.requestId, 'gen-1');
});

test('network failure still reports the trace id it attempted', async () => {
  const tp = '00-' + 'd'.repeat(32) + '-' + 'e'.repeat(16) + '-01';
  const client = new DaemonClient({
    baseUrl: 'http://example.invalid',
    fetch: async () => {
      throw new Error('connect refused');
    },
  });
  const result = await client.deploy({ traceparent: tp, requestId: 'req-9' });
  assert.equal(result.ok, false);
  assert.equal(result.traceId, 'd'.repeat(32));
  assert.equal(result.requestId, 'req-9');
});

test('routerHeaders and vaultSecretManagerHeaders build the contract headers', () => {
  const { routerHeaders, vaultSecretManagerHeaders, CREDENTIALS } = require('./index.js');

  assert.deepEqual(routerHeaders({ apiToken: 'tok', zoneId: 'z1', accountId: 'a1' }), {
    'X-Router-Api-Token': 'tok',
    'X-Router-Zone-Id': 'z1',
    'X-Router-Account-Id': 'a1',
  });
  assert.deepEqual(routerHeaders(undefined), {});
  assert.deepEqual(routerHeaders({ zoneId: 'z1' }), { 'X-Router-Zone-Id': 'z1' });

  assert.deepEqual(vaultSecretManagerHeaders({ token: 'hvs.1', addr: 'https://v:8200', namespace: 'ns' }), {
    'X-Secret-Vault-Token': 'hvs.1',
    'X-Secret-Vault-Addr': 'https://v:8200',
    'X-Secret-Vault-Namespace': 'ns',
  });
  // Token is the anchor: no token, no headers (a bare addr is useless).
  assert.deepEqual(vaultSecretManagerHeaders({ addr: 'https://v:8200' }), {});

  // Helpers stay aligned with the shipped contract.
  const routerContract = CREDENTIALS.router.map((c) => c.header);
  assert.deepEqual(routerContract, ['X-Router-Api-Token', 'X-Router-Zone-Id', 'X-Router-Account-Id']);
  const vaultContract = CREDENTIALS.secretManagers['vault-secret-manager'].map((c) => c.header);
  assert.ok(vaultContract.includes('X-Secret-Vault-Token'));
});

test('scoped AWS identity helpers (state + secret manager) stay isolated from provider headers', () => {
  const { stateAwsHeaders, awsSecretManagerHeaders, CREDENTIALS } = require('./index.js');

  const state = stateAwsHeaders({ accessKeyId: 'AKIA_STATE', secretAccessKey: 's', sessionToken: 't', region: 'eu-central-1' });
  assert.deepEqual(state, {
    'X-State-Aws-Access-Key-Id': 'AKIA_STATE',
    'X-State-Aws-Secret-Access-Key': 's',
    'X-State-Aws-Session-Token': 't',
    'X-State-Aws-Region': 'eu-central-1',
  });
  const sm = awsSecretManagerHeaders({ accessKeyId: 'AKIA_SM', secretAccessKey: 's2' });
  assert.deepEqual(sm, {
    'X-Secret-Aws-Access-Key-Id': 'AKIA_SM',
    'X-Secret-Aws-Secret-Access-Key': 's2',
  });
  // Partial identities produce nothing.
  assert.deepEqual(stateAwsHeaders({ accessKeyId: 'only' }), {});
  assert.deepEqual(awsSecretManagerHeaders(undefined), {});

  // The three AWS identities target disjoint header namespaces.
  const provider = new Set(Object.keys(require('./index.js').awsProviderHeaders({ accessKeyId: 'a', secretAccessKey: 'b' })));
  for (const k of [...Object.keys(state), ...Object.keys(sm)]) {
    assert.ok(!provider.has(k), `header ${k} must not collide with provider headers`);
  }

  // Contract ships both scoped groups.
  assert.ok(CREDENTIALS.stateAws.some((c) => c.header === 'X-State-Aws-Access-Key-Id'));
  assert.ok(CREDENTIALS.secretManagers['aws-secret-manager'].some((c) => c.header === 'X-Secret-Aws-Access-Key-Id'));
  // Router plugin declarations ship with DECLARATIVE same-cloud fallbacks
  // (env-only; headers are the shared router group).
  const cfToken = CREDENTIALS.routerPlugins.cloudflare.find((c) => c.envKey === 'RUNFABRIC_ROUTER_API_TOKEN');
  assert.equal(cfToken.fallback, 'CLOUDFLARE_API_TOKEN');
  const ns1Token = CREDENTIALS.routerPlugins.ns1.find((c) => c.envKey === 'RUNFABRIC_ROUTER_API_TOKEN');
  assert.equal(ns1Token.fallback, 'NS1_API_KEY');
  assert.equal(CREDENTIALS.routerPlugins.route53, undefined); // AWS default chain — declares none
});

test('fabricDeploy and routerSync hit the multi-cloud routes with provider/dryRun params', async () => {
  const stub = await startStub({
    '/fabric/deploy': { status: 200, body: { service: 's', endpoints: [{ provider: 'aws', url: 'https://a' }, { provider: 'gcp', url: 'https://g' }] } },
    '/router/sync': { status: 200, body: { routing: { strategy: 'failover' }, result: { actions: [] } } },
    '/deploy': { status: 200, body: { deploymentId: 'dpl-3' } },
  });
  try {
    const client = new DaemonClient({ baseUrl: stub.baseUrl });

    const fab = await client.fabricDeploy({ configPath: 't/p/runfabric.yml', stage: 'prod' });
    assert.equal(fab.ok, true);
    assert.equal(fab.data.endpoints.length, 2);

    const sync = await client.routerSync({ stage: 'prod', dryRun: true });
    assert.equal(sync.ok, true);

    // Single-target multi-cloud selection rides the provider query param.
    const dep = await client.deploy({ stage: 'prod', provider: 'gcp' });
    assert.equal(dep.ok, true);

    assert.equal(stub.seen[0].path, '/fabric/deploy');
    assert.equal(stub.seen[1].path, '/router/sync');
    assert.equal(stub.seen[1].query.dryRun, '1');
    assert.equal(stub.seen[2].query.provider, 'gcp');
  } finally {
    await stub.close();
  }
});

test('ops endpoints: invoke body, op params, state/router op paths', async () => {
  const stub = await startStub({
    '/invoke': { status: 200, body: { result: 'pong' } },
    '/logs': { status: 200, body: { entries: [] } },
    '/metrics/functions': { status: 200, body: { functions: {} } },
    '/doctor': { status: 200, body: { backend: {}, provider: {} } },
    '/recover': { status: 200, body: { recovered: true } },
    '/fabric/health': { status: 200, body: { endpoints: [] } },
    '/fabric/targets': { status: 200, body: { targets: ['aws', 'gcp'] } },
    '/state/backup': { status: 200, body: { path: 'b.json' } },
    '/router/shift': { status: 200, body: { weights: {} } },
  });
  try {
    const client = new DaemonClient({ baseUrl: stub.baseUrl });

    const inv = await client.invoke({ stage: 'prod', function: 'hello', payload: { name: 'x' } });
    assert.equal(inv.ok, true);
    await client.logs({ stage: 'prod', function: 'hello', service: 'api' });
    await client.functionMetrics({ stage: 'prod', all: true });
    await client.doctor({ stage: 'prod', provider: 'aws' });
    await client.recover({ stage: 'prod', mode: 'inspect', dryRun: true });
    await client.fabricHealth({ stage: 'prod' });
    const targets = await client.fabricTargets({ stage: 'prod' });
    assert.deepEqual(targets.data.targets, ['aws', 'gcp']);
    await client.stateOp('backup', { stage: 'prod', params: { out: 'backups/s.json' } });
    await client.routerOp('shift', { stage: 'prod', provider: 'gcp', params: { percent: 20 }, dryRun: true });

    assert.equal(stub.seen[0].path, '/invoke');
    assert.equal(stub.seen[0].query.function, 'hello');
    assert.equal(stub.seen[0].headers['content-type'], 'application/json');
    assert.equal(stub.seen[1].query.service, 'api');
    assert.equal(stub.seen[2].path, '/metrics/functions');
    assert.equal(stub.seen[2].query.all, '1');
    assert.equal(stub.seen[3].query.provider, 'aws');
    assert.equal(stub.seen[4].query.mode, 'inspect');
    assert.equal(stub.seen[4].query.dryRun, '1');
    assert.equal(stub.seen[5].path, '/fabric/health');
    assert.equal(stub.seen[7].path, '/state/backup');
    assert.equal(stub.seen[7].query.out, 'backups/s.json');
    assert.equal(stub.seen[8].path, '/router/shift');
    assert.equal(stub.seen[8].query.provider, 'gcp');
    assert.equal(stub.seen[8].query.percent, '20');
    assert.equal(stub.seen[8].query.dryRun, '1');
  } finally {
    await stub.close();
  }
});

test('workflow ops: run body + params, status/cancel/replay/runs paths', async () => {
  const stub = await startStub({
    '/workflow/run': { status: 200, body: { workflow: 'etl', run: { id: 'r1', status: 'running' } } },
    '/workflow/status': { status: 200, body: { id: 'r1', status: 'succeeded' } },
    '/workflow/cancel': { status: 200, body: { id: 'r1', status: 'cancelled' } },
    '/workflow/replay': { status: 200, body: { id: 'r1', status: 'running' } },
    '/workflow/approve': { status: 200, body: { id: 'r1', status: 'running' } },
    '/workflow/runs': { status: 200, body: [{ id: 'r1' }] },
  });
  try {
    const client = new DaemonClient({ baseUrl: stub.baseUrl });

    const run = await client.workflowRun({ stage: 'prod', name: 'etl', payload: { day: '2026-07-06' } });
    assert.equal(run.ok, true);
    await client.workflowStatus({ stage: 'prod', runId: 'r1' });
    await client.workflowCancel({ stage: 'prod', runId: 'r1' });
    await client.workflowReplay({ stage: 'prod', runId: 'r1', step: 'extract' });
    await client.workflowApprove({ stage: 'prod', runId: 'r1', decision: 'approve', reviewer: 'alice' });
    const runs = await client.workflowRuns({ stage: 'prod', limit: 5 });
    assert.equal(runs.ok, true);

    assert.equal(stub.seen[0].path, '/workflow/run');
    assert.equal(stub.seen[0].query.name, 'etl');
    assert.equal(stub.seen[0].headers['content-type'], 'application/json');
    assert.equal(stub.seen[1].query.runId, 'r1');
    assert.equal(stub.seen[2].path, '/workflow/cancel');
    assert.equal(stub.seen[3].query.step, 'extract');
    assert.equal(stub.seen[4].path, '/workflow/approve');
    assert.equal(stub.seen[4].query.decision, 'approve');
    assert.equal(stub.seen[4].query.reviewer, 'alice');
    assert.equal(stub.seen[5].query.limit, '5');
  } finally {
    await stub.close();
  }
});

test('extensions() GETs /extensions and returns the catalog', async () => {
  const catalog = {
    kinds: [
      {
        kind: 'provider',
        configKey: 'provider.name',
        plugins: [
          {
            id: 'cloudflare-workers',
            source: 'builtin',
            supportsRuntime: ['nodejs', 'python'],
            supportsTriggers: ['cron', 'http'],
            credentials: [{ envKey: 'CLOUDFLARE_API_TOKEN', required: true }],
            scaffold: { comment: 'x', entry: 'worker.fetch', entryFile: 'worker.js' },
          },
        ],
      },
    ],
  };
  const stub = await startStub({ '/extensions': { status: 200, body: catalog } });
  try {
    const client = new DaemonClient({ baseUrl: stub.baseUrl, apiKey: 'secret' });
    const result = await client.extensions();
    assert.equal(result.ok, true);
    assert.equal(result.data.kinds[0].plugins[0].scaffold.entryFile, 'worker.js');
    const req = stub.seen[0];
    assert.equal(req.method, 'GET');
    assert.equal(req.path, '/extensions');
    assert.equal(req.headers['x-api-key'], 'secret');
  } finally {
    await stub.close();
  }
});

test('scaffold() POSTs /scaffold with option query params; returns files/entry/runtime', async () => {
  const stub = await startStub({
    '/scaffold': {
      status: 200,
      body: { ok: true, files: { 'runfabric.yml': 'service: demo\n' }, entry: 'worker.fetch', runtime: 'nodejs20.x' },
    },
  });
  try {
    const client = new DaemonClient({ baseUrl: stub.baseUrl });
    const result = await client.scaffold({
      provider: 'cloudflare-workers',
      template: 'http',
      lang: 'js',
      stateBackend: 's3',
      service: 'demo',
      withBuild: false,
    });
    assert.equal(result.ok, true);
    assert.equal(result.data.entry, 'worker.fetch');
    assert.ok('runfabric.yml' in result.data.files);
    const req = stub.seen[0];
    assert.equal(req.method, 'POST');
    assert.equal(req.path, '/scaffold');
    assert.equal(req.query.provider, 'cloudflare-workers');
    assert.equal(req.query.stateBackend, 's3');
    assert.equal(req.query.withBuild, 'false');
  } finally {
    await stub.close();
  }
});

test('scaffold() maps 422 to ok:false with the daemon error', async () => {
  const stub = await startStub({
    '/scaffold': { status: 422, body: { ok: false, error: 'provider "fly-machines" does not support trigger "pubsub"' } },
  });
  try {
    const client = new DaemonClient({ baseUrl: stub.baseUrl });
    const result = await client.scaffold({ provider: 'fly-machines', template: 'pubsub', lang: 'js', service: 'x' });
    assert.equal(result.ok, false);
    assert.equal(result.status, 422);
    assert.match(result.error, /does not support trigger/);
  } finally {
    await stub.close();
  }
});

test('invokeLocal() POSTs /invoke-local with the inline project + request in the body', async () => {
  const stub = await startStub({
    '/invoke-local': {
      status: 200,
      body: {
        ok: true,
        function: 'api',
        runtime: 'nodejs20.x',
        simulated: true,
        statusCode: 200,
        headers: { 'Content-Type': 'application/json' },
        body: { message: 'Hello from RunFabric' },
      },
    },
  });
  try {
    const client = new DaemonClient({ baseUrl: stub.baseUrl });
    const result = await client.invokeLocal({
      runfabricYaml: 'service: demo\n',
      handlerCode: 'exports.handler = async () => ({ statusCode: 200 });',
      function: 'api',
      stage: 'dev',
      request: { method: 'POST', path: '/api', body: '{"x":1}' },
    });
    assert.equal(result.ok, true);
    assert.equal(result.data.simulated, true);
    assert.equal(result.data.statusCode, 200);
    assert.equal(result.data.body.message, 'Hello from RunFabric');
    const req = stub.seen[0];
    assert.equal(req.method, 'POST');
    assert.equal(req.path, '/invoke-local');
    // The whole project rides the JSON body, not query params.
    assert.equal(req.body.runfabricYaml, 'service: demo\n');
    assert.equal(req.body.function, 'api');
    assert.equal(req.body.request.method, 'POST');
    assert.equal(req.body.request.body, '{"x":1}');
  } finally {
    await stub.close();
  }
});

test('invokeLocal() maps 422 to ok:false with the daemon error', async () => {
  const stub = await startStub({
    '/invoke-local': { status: 422, body: { ok: false, error: 'bootstrap: invalid config' } },
  });
  try {
    const client = new DaemonClient({ baseUrl: stub.baseUrl });
    const result = await client.invokeLocal({ runfabricYaml: 'nope' });
    assert.equal(result.ok, false);
    assert.equal(result.status, 422);
    assert.match(result.error, /bootstrap/);
  } finally {
    await stub.close();
  }
});
