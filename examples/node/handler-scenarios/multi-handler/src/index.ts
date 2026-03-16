import type { UniversalHandler } from "@runfabric/core";

export const handler: UniversalHandler = async () => ({
  status: 200,
  headers: { "content-type": "application/json" },
  body: JSON.stringify({
    service: "handler-scenario-multi",
    route: "/health",
    ok: true
  })
});
