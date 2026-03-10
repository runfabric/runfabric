import type { UniversalHandler } from "@runfabric/core";

export function createVercelHttpAdapter(handler: UniversalHandler) {
  return async function vercelHttpHandler(req: any, res?: any) {
    const url = req?.url ? new URL(req.url, "https://localhost") : undefined;
    const response = await handler({
      method: req?.method || "GET",
      path: req?.path || url?.pathname || "/",
      headers: req?.headers || {},
      query: req?.query || Object.fromEntries(url?.searchParams?.entries?.() || []),
      body: typeof req?.body === "string" ? req.body : req?.body ? JSON.stringify(req.body) : undefined,
      raw: req
    });

    if (res && typeof res.status === "function" && typeof res.send === "function") {
      if (response.headers && typeof res.setHeader === "function") {
        for (const [key, value] of Object.entries(response.headers)) {
          res.setHeader(key, value);
        }
      }
      res.status(response.status).send(response.body || "");
      return;
    }

    return {
      statusCode: response.status,
      headers: response.headers || {},
      body: response.body || ""
    };
  };
}
