import type { UniversalHandler } from "@runfabric/core";

export const handler: UniversalHandler = async (req) => ({
  status: 202,
  headers: { "content-type": "application/json" },
  body: JSON.stringify({
    function: "webhook",
    accepted: true,
    method: req.method,
    path: req.path
  })
});
