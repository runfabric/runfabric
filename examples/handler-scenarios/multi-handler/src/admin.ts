import type { UniversalHandler } from "@runfabric/core";

function parseJson(body: string | undefined): unknown {
  if (!body) {
    return null;
  }
  try {
    return JSON.parse(body);
  } catch {
    return body;
  }
}

export const handler: UniversalHandler = async (req) => ({
  status: 200,
  headers: { "content-type": "application/json" },
  body: JSON.stringify({
    function: "admin-api",
    method: req.method,
    path: req.path,
    body: parseJson(req.body)
  })
});
