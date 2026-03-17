import type { UniversalHandler } from "@runfabric/sdk";

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
