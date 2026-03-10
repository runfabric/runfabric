import type { UniversalHandler } from "@runfabric/core";

export function createAlibabaHttpAdapter(handler: UniversalHandler) {
  return async function alibabaHttpHandler(event: any) {
    const response = await handler({
      method: event?.httpMethod || event?.method || "GET",
      path: event?.path || "/",
      headers: event?.headers || {},
      query: event?.queryParameters || {},
      body: event?.body,
      raw: event
    });

    return {
      statusCode: response.status,
      headers: response.headers || {},
      body: response.body || ""
    };
  };
}
