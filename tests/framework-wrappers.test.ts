import test from "node:test";
import assert from "node:assert/strict";
import type { IncomingMessage, ServerResponse } from "node:http";
import { createHandler } from "../packages/runtime-node/src/framework-wrappers.ts";

test("createHandler detects express app and proxies request", async () => {
  const app = async (req: IncomingMessage, res: ServerResponse): Promise<void> => {
    const chunks: Buffer[] = [];
    for await (const chunk of req) {
      chunks.push(Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk));
    }

    const body = Buffer.concat(chunks).toString("utf8");
    const responseBody = {
      method: req.method,
      url: req.url,
      xTest: req.headers["x-test"],
      body
    };

    res.statusCode = 200;
    res.setHeader("content-type", "application/json");
    res.end(JSON.stringify(responseBody));
  };

  const handler = createHandler(app);
  const response = await handler({
    method: "POST",
    path: "/hello",
    headers: {
      "content-type": "application/json",
      "x-test": "express"
    },
    query: {
      page: "1",
      tag: ["one", "two"]
    },
    body: JSON.stringify({ ok: true })
  });

  assert.equal(response.status, 200);
  assert.equal(response.headers?.["content-type"], "application/json");

  const parsed = JSON.parse(response.body ?? "{}") as {
    method?: string;
    url?: string;
    xTest?: string;
    body?: string;
  };
  assert.equal(parsed.method, "POST");
  assert.equal(parsed.url, "/hello?page=1&tag=one&tag=two");
  assert.equal(parsed.xTest, "express");
  assert.equal(parsed.body, "{\"ok\":true}");
});

test("createHandler detects fastify and maps request shape to inject()", async () => {
  let captured: {
    method: string;
    url: string;
    headers?: Record<string, string>;
    payload?: unknown;
  } | undefined;

  const app = {
    inject: async (options: {
      method: string;
      url: string;
      headers?: Record<string, string>;
      payload?: unknown;
    }) => {
      captured = options;
      return {
        statusCode: 201,
        headers: {
          "x-wrapper": "fastify"
        },
        payload: JSON.stringify({
          url: options.url,
          payload: options.payload
        })
      };
    }
  };

  const handler = createHandler(app);
  const response = await handler({
    method: "PUT",
    path: "/jobs",
    headers: {
      "content-type": "application/json",
      "x-test": "fastify"
    },
    query: {
      env: "dev",
      key: ["a", "b"]
    },
    body: "{\"job\":\"sync\"}"
  });

  assert.equal(captured?.method, "PUT");
  assert.equal(captured?.url, "/jobs?env=dev&key=a&key=b");
  assert.equal(captured?.headers?.["x-test"], "fastify");
  assert.deepEqual(captured?.payload, { job: "sync" });

  assert.equal(response.status, 201);
  assert.equal(response.headers?.["x-wrapper"], "fastify");
  const parsed = JSON.parse(response.body ?? "{}") as { payload?: unknown };
  assert.deepEqual(parsed.payload, { job: "sync" });
});

test("createHandler detects nest app and routes to fastify instance", async () => {
  const app = {
    inject: async () => ({
      statusCode: 202,
      headers: { "x-adapter": "fastify" },
      body: "ok-fastify"
    })
  };

  const handler = createHandler({
    getHttpAdapter: () => ({
      getType: () => "fastify",
      getInstance: () => app
    })
  });

  const response = await handler({
    method: "GET",
    path: "/health",
    headers: {},
    query: {}
  });

  assert.equal(response.status, 202);
  assert.equal(response.body, "ok-fastify");
});

test("createHandler detects nest app and routes to express instance", async () => {
  const app = (_req: IncomingMessage, res: ServerResponse): void => {
    res.statusCode = 204;
    res.end();
  };

  const handler = createHandler({
    getHttpAdapter: () => ({
      getType: () => "express",
      getInstance: () => app
    })
  });

  const response = await handler({
    method: "GET",
    path: "/health",
    headers: {},
    query: {}
  });

  assert.equal(response.status, 204);
});

test("createHandler throws for unsupported nest adapter", () => {
  assert.throws(
    () =>
      createHandler({
        getHttpAdapter: () => ({
          getType: () => "custom",
          getInstance: () => ({})
        })
      }),
    /unsupported nest http adapter type: custom/
  );
});

test("createHandler throws for unsupported app", () => {
  assert.throws(() => createHandler({}), /createHandler expects one of/);
});
