import type { UniversalHandler } from "@runfabric/core";

export const handler: UniversalHandler = async (req) => ({
  status: 200,
  headers: { "content-type": "application/json" },
  body: JSON.stringify({
    function: "public-api",
    method: req.method,
    path: req.path
  })
});
