import type { UniversalHandler } from "@runfabric/core";

export function createAwsAdapter(handler: UniversalHandler) {
  return async function awsHandler(event: any) {
    const method = event.requestContext?.http?.method || event.httpMethod || "GET";
    const path = event.rawPath || event.path || "/";
    const body =
      event.isBase64Encoded && typeof event.body === "string"
        ? Buffer.from(event.body, "base64").toString("utf8")
        : event.body;

    const res = await handler({
      method,
      path,
      headers: event.headers || {},
      query: event.queryStringParameters || {},
      body,
      raw: event
    });

    return {
      statusCode: res.status,
      headers: res.headers || {},
      body: res.body || ""
    };
  };
}
