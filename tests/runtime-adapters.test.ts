import test from "node:test";
import assert from "node:assert/strict";
import { createCloudflareWorkerAdapter } from "../packages/runtime-node/src/cloudflare-worker-adapter.ts";
import { createDigitalOceanHttpAdapter } from "../packages/runtime-node/src/digitalocean-http-adapter.ts";
import { createAzureHttpAdapter } from "../packages/runtime-node/src/azure-http-adapter.ts";
import { createGcpHttpAdapter } from "../packages/runtime-node/src/gcp-http-adapter.ts";
import { createVercelHttpAdapter } from "../packages/runtime-node/src/vercel-http-adapter.ts";

test("gcp adapter maps request to universal handler shape", async () => {
  const adapter = createGcpHttpAdapter(async (req) => {
    return {
      status: 200,
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        method: req.method,
        path: req.path
      })
    };
  });

  const response = await adapter({
    method: "POST",
    path: "/users",
    headers: { "x-test": "1" },
    query: { page: "1" },
    body: { ok: true }
  });

  assert.equal(response?.statusCode, 200);
  assert.ok(response?.body.includes('"path":"/users"'));
});

test("azure adapter sets context response", async () => {
  const adapter = createAzureHttpAdapter(async (req) => {
    return {
      status: 201,
      headers: { "x-adapter": "azure" },
      body: JSON.stringify({ method: req.method })
    };
  });

  const context: { res?: unknown } = {};
  const result = await adapter(context, {
    method: "GET",
    url: "/health",
    headers: {},
    query: {}
  });

  assert.equal((result as { status: number }).status, 201);
  assert.ok(context.res);
});

test("vercel adapter returns node-style response when res object is absent", async () => {
  const adapter = createVercelHttpAdapter(async (req) => ({
    status: 202,
    headers: { "x-provider": "vercel" },
    body: JSON.stringify({ path: req.path })
  }));

  const result = await adapter({
    method: "GET",
    url: "/hello?name=runfabric",
    headers: {}
  });

  assert.equal(result?.statusCode, 202);
  assert.ok(result?.body.includes("/hello"));
});

test("digitalocean adapter maps __ow payload fields", async () => {
  const adapter = createDigitalOceanHttpAdapter(async (req) => ({
    status: 200,
    body: JSON.stringify({ method: req.method, path: req.path })
  }));

  const result = await adapter({
    __ow_method: "POST",
    __ow_path: "/jobs"
  });

  assert.equal(result.statusCode, 200);
  assert.ok(result.body.includes("\"path\":\"/jobs\""));
});

test("cloudflare worker adapter exposes fetch()", async () => {
  const adapter = createCloudflareWorkerAdapter(async () => ({
    status: 200,
    headers: { "content-type": "text/plain" },
    body: "ok"
  }));

  const response = await adapter.fetch(new Request("https://example.com/health", { method: "GET" }));
  assert.equal(response.status, 200);
  assert.equal(await response.text(), "ok");
});
