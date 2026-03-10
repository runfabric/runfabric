import type { UniversalHandler } from "@runfabric/core";

export function createNetlifyHttpAdapter(handler: UniversalHandler) {
  return async function netlifyHttpHandler(event: any) {
    const method = event?.httpMethod || event?.requestContext?.http?.method || "GET";
    const path = event?.path || event?.rawPath || "/";
    const response = await handler({
      method,
      path,
      headers: event?.headers || {},
      query: event?.queryStringParameters || {},
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
