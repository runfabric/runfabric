import test from "node:test";
import assert from "node:assert/strict";
import {
  createHandlerResolver,
  mergeServeEventTemplate
} from "../apps/cli/src/commands/call-local/serve.ts";

test("createHandlerResolver returns static handler when watch mode is disabled", async () => {
  const resolver = createHandlerResolver<string>({
    watchMode: false,
    initialHandler: "static-handler",
    loadFreshHandler: async () => "fresh-handler",
    readWatchVersion: async () => "v1"
  });

  assert.equal(await resolver(), "static-handler");
  assert.equal(await resolver(), "static-handler");
});

test("createHandlerResolver caches watch handler for unchanged version", async () => {
  let version = "v1";
  let loadCount = 0;
  const resolver = createHandlerResolver<string>({
    watchMode: true,
    loadFreshHandler: async () => {
      loadCount += 1;
      return `handler-${loadCount}`;
    },
    readWatchVersion: async () => version
  });

  assert.equal(await resolver(), "handler-1");
  assert.equal(await resolver(), "handler-1");
  assert.equal(loadCount, 1);

  version = "v2";
  assert.equal(await resolver(), "handler-2");
  assert.equal(loadCount, 2);
});

test("createHandlerResolver coalesces concurrent watch loads", async () => {
  let loadCount = 0;
  const resolver = createHandlerResolver<string>({
    watchMode: true,
    loadFreshHandler: async () => {
      loadCount += 1;
      await new Promise((resolvePromise) => setTimeout(resolvePromise, 10));
      return "handler";
    },
    readWatchVersion: async () => "v1"
  });

  const [first, second] = await Promise.all([resolver(), resolver()]);
  assert.equal(first, "handler");
  assert.equal(second, "handler");
  assert.equal(loadCount, 1);
});

test("mergeServeEventTemplate keeps template fields and overrides request keys", () => {
  const merged = mergeServeEventTemplate(
    {
      requestContext: {
        authorizer: {
          principalId: "template-user"
        },
        http: {
          method: "POST"
        }
      },
      rawPath: "/from-template"
    },
    {
      requestContext: {
        http: {
          method: "GET"
        }
      },
      rawPath: "/from-request"
    }
  ) as Record<string, unknown>;

  assert.equal((merged.rawPath as string), "/from-request");
  assert.deepEqual(merged.requestContext, {
    authorizer: { principalId: "template-user" },
    http: { method: "GET" }
  });
});
