/**
 * Binds an existing Express, Fastify, or Nest app to the Lambda/RunFabric invocation model.
 * Universal handler contract: (request: UniversalRequest) => Promise<UniversalResponse>.
 * Express via real HTTP server + fetch; Fastify via inject(); Nest via getHttpAdapter().
 * @module @runfabric/sdk/adapters/universalHandler
 */

const { once } = require("node:events");
const { createServer } = require("node:http");

/**
 * @typedef {Object} UniversalRequest
 * @property {string} [method]
 * @property {string} [path]
 * @property {Record<string, string|string[]>} [query]
 * @property {Record<string, string>} [headers]
 * @property {string|undefined} [body]
 */

/**
 * @typedef {Object} UniversalResponse
 * @property {number} status
 * @property {Record<string, string>} [headers]
 * @property {string} [body]
 */

function normalizeHeaders(headers) {
    const out = {};
    if (!headers || typeof headers !== "object") return out;
    for (const [key, value] of Object.entries(headers)) {
        if (typeof value === "string") out[key] = value;
        else if (Array.isArray(value)) out[key] = value.map(String).join(", ");
        else if (value != null) out[key] = String(value);
    }
    return out;
}

function buildUrlPath(path, query) {
    if (!query || typeof query !== "object") return path || "/";
    const params = new URLSearchParams();
    for (const [key, value] of Object.entries(query)) {
        if (Array.isArray(value)) value.forEach((v) => params.append(key, v));
        else params.set(key, value);
    }
    const q = params.toString();
    return q ? `${path || "/"}?${q}` : path || "/";
}

function toPayload(body, headers) {
    if (body === undefined || body === null) return undefined;
    if (typeof body === "object") return JSON.stringify(body);
    return String(body);
}

function isFastifyInstance(value) {
    return value && typeof value === "object" && typeof value.inject === "function";
}

function isNestApplication(value) {
    return (
        value &&
        typeof value === "object" &&
        typeof value.getHttpAdapter === "function"
    );
}

function isExpressApplication(value) {
    return (
        typeof value === "function" &&
        typeof value.use === "function" &&
        typeof value.listen === "function"
    );
}

const expressState = { current: null };

async function ensureExpressServer(app) {
    if (expressState.current) return expressState.current;

    const server = createServer((req, res) => {
        const onError = (err) => {
            const msg = err instanceof Error ? err.message : String(err);
            res.statusCode = 500;
            res.setHeader("content-type", "application/json");
            res.end(JSON.stringify({ error: msg }));
        };
        try {
            const result = app(req, res, (err) => {
                if (err) onError(err);
            });
            if (result && typeof result.then === "function") result.catch(onError);
        } catch (e) {
            onError(e);
        }
    });

    server.listen(0, "127.0.0.1");
    await once(server, "listening");
    server.unref();

    const addr = server.address();
    if (!addr || typeof addr !== "object") {
        throw new Error("failed to bind express wrapper server");
    }
    expressState.current = {
        server,
        origin: `http://127.0.0.1:${addr.port}`,
    };
    return expressState.current;
}

function createExpressHandler(app) {
    if (!isExpressApplication(app)) {
        throw new Error("createHandler expects an express-compatible app (function with .use and .listen)");
    }

    return async (request) => {
        const server = await ensureExpressServer(app);
        const urlPath = buildUrlPath(request.path, request.query);
        const headers = normalizeHeaders(request.headers);
        const body = request.body;
        const bodyStr =
            body === undefined || body === null
                ? undefined
                : typeof body === "string"
                  ? body
                  : JSON.stringify(body);

        const res = await fetch(`${server.origin}${urlPath}`, {
            method: (request.method || "GET").toUpperCase(),
            headers: { "content-type": "application/json", ...headers },
            body: bodyStr,
        });

        const outHeaders = {};
        res.headers.forEach((v, k) => {
            outHeaders[k] = v;
        });
        return {
            status: res.status,
            headers: outHeaders,
            body: await res.text(),
        };
    };
}

function createFastifyHandler(app) {
    if (!isFastifyInstance(app)) {
        throw new Error("createHandler expects a fastify-compatible instance with inject()");
    }

    return async (request) => {
        const headers = normalizeHeaders(request.headers);
        const urlPath = buildUrlPath(request.path, request.query);
        const injected = await app.inject({
            method: (request.method || "GET").toUpperCase(),
            url: urlPath,
            headers,
            payload: toPayload(request.body, headers),
        });

        const body =
            typeof injected.body === "string"
                ? injected.body
                : typeof injected.payload === "string"
                  ? injected.payload
                  : "";

        return {
            status: injected.statusCode || 200,
            headers: normalizeHeaders(injected.headers),
            body,
        };
    };
}

function createNestHandler(app) {
    if (!app || typeof app.getHttpAdapter !== "function") {
        throw new Error("createHandler expects a nest application with getHttpAdapter()");
    }

    const adapter = app.getHttpAdapter();
    const adapterType = typeof adapter.getType === "function" ? adapter.getType() : undefined;
    const instance = adapter.getInstance && adapter.getInstance();

    if (adapterType === "fastify" || isFastifyInstance(instance)) {
        return createFastifyHandler(instance);
    }
    if (adapterType === "express" || isExpressApplication(instance)) {
        return createExpressHandler(instance);
    }
    throw new Error(`unsupported nest http adapter type: ${adapterType || "unknown"}`);
}

/**
 * Create a UniversalHandler from an Express/Fastify/Nest app.
 * Handler signature: (request: UniversalRequest) => Promise<UniversalResponse>.
 * @param {unknown} app - Express app (function with .use/.listen), Fastify instance (inject), or Nest app (getHttpAdapter)
 * @returns {(request: UniversalRequest) => Promise<UniversalResponse>}
 */
function createHandler(app) {
    if (isNestApplication(app)) return createNestHandler(app);
    if (isFastifyInstance(app)) return createFastifyHandler(app);
    if (isExpressApplication(app)) return createExpressHandler(app);

    throw new Error(
        "createHandler expects one of: nest app (getHttpAdapter), fastify instance (inject), or express app (function with .use and .listen)"
    );
}

module.exports = {
    createHandler,
    createExpressHandler,
    createFastifyHandler,
    createNestHandler,
    isExpressApplication,
    isFastifyInstance,
    isNestApplication,
    normalizeHeaders,
    buildUrlPath,
};
