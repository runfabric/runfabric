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
    seen.push({
      method: req.method,
      path: url.pathname,
      query: Object.fromEntries(url.searchParams),
      headers: req.headers,
    });
    const route = routes[url.pathname] || { status: 404, body: { ok: false, error: 'not found' } };
    res.writeHead(route.status, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify(route.body));
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
