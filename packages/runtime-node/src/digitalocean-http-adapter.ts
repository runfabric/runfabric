import type { UniversalHandler } from "@runfabric/core";

export function createDigitalOceanHttpAdapter(handler: UniversalHandler) {
  return async function digitalOceanHttpHandler(args: any) {
    const response = await handler({
      method: args?.__ow_method || args?.method || "GET",
      path: args?.__ow_path || args?.path || "/",
      headers: args?.__ow_headers || args?.headers || {},
      query: args?.__ow_query || args?.query || {},
      body: args?.__ow_body || args?.body,
      raw: args
    });

    return {
      statusCode: response.status,
      headers: response.headers || {},
      body: response.body || ""
    };
  };
}
