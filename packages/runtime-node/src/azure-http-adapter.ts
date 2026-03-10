import type { UniversalHandler } from "@runfabric/core";

export function createAzureHttpAdapter(handler: UniversalHandler) {
  return async function azureHttpHandler(context: any, req: any) {
    const response = await handler({
      method: req?.method || "GET",
      path: req?.url || req?.originalUrl || "/",
      headers: req?.headers || {},
      query: req?.query || {},
      body:
        typeof req?.rawBody === "string"
          ? req.rawBody
          : typeof req?.body === "string"
            ? req.body
            : req?.body
              ? JSON.stringify(req.body)
              : undefined,
      raw: { context, req }
    });

    const azureResponse = {
      status: response.status,
      headers: response.headers || {},
      body: response.body || ""
    };

    if (context) {
      context.res = azureResponse;
    }

    return azureResponse;
  };
}
