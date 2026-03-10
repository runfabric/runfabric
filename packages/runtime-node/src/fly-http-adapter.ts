import type { UniversalHandler } from "@runfabric/core";

export function createFlyHttpAdapter(handler: UniversalHandler) {
  return async function flyHttpHandler(req: any, res?: any) {
    const response = await handler({
      method: req?.method || "GET",
      path: req?.path || req?.url || "/",
      headers: req?.headers || {},
      query: req?.query || {},
      body: typeof req?.body === "string" ? req.body : req?.body ? JSON.stringify(req.body) : undefined,
      raw: req
    });

    if (res && typeof res.status === "function" && typeof res.send === "function") {
      if (response.headers && typeof res.set === "function") {
        for (const [key, value] of Object.entries(response.headers)) {
          res.set(key, value);
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
