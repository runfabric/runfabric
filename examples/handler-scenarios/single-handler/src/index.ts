import type { UniversalHandler } from "@runfabric/core";

export const handler: UniversalHandler = async (req) => ({
  status: 200,
  headers: { "content-type": "application/json" },
  body: JSON.stringify({
    message: "hello from single handler",
    method: req.method,
    path: req.path,
    query: req.query
  })
});
