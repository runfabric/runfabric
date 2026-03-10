import type { UniversalHandler } from "@runfabric/core";

export function createIbmOpenWhiskAdapter(handler: UniversalHandler) {
  return async function ibmOpenWhiskHandler(params: any) {
    const response = await handler({
      method: params?.__ow_method || params?.method || "GET",
      path: params?.__ow_path || params?.path || "/",
      headers: params?.__ow_headers || params?.headers || {},
      query: params?.__ow_query || params?.query || {},
      body: params?.__ow_body || params?.body,
      raw: params
    });

    return {
      statusCode: response.status,
      headers: response.headers || {},
      body: response.body || ""
    };
  };
}
