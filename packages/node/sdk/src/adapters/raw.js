/**
 * Raw handler support for Node.
 * Wrap a function (event, context) => response for use with runfabric call-local or provider invocation.
 * @module @runfabric/sdk/adapters/raw
 */

/**
 * Standard handler signature: (event, context) => response
 * @typedef {Object} Context
 * @property {string} [requestId]
 * @property {string} [stage]
 * @property {string} [functionName]
 */
/**
 * @typedef {(event: object, context: Context) => Promise<object> | object} RawHandler
 */

/**
 * Wraps a raw handler so it can be invoked with (req, res) in HTTP mode.
 * For use with runfabric call-local or custom servers.
 * @param {RawHandler} handler - (event, context) => response
 * @returns {(req: import('http').IncomingMessage, res: import('http').ServerResponse) => void}
 */
function createHttpHandler(handler) {
    return (req, res) => {
        const chunks = [];
        req.on("data", (c) => chunks.push(c));
        req.on("end", () => {
            const body = Buffer.concat(chunks).toString("utf8") || "{}";
            let event = {};
            try {
                event = JSON.parse(body);
            } catch (_) {}
            const context = {
                requestId: req.headers["x-request-id"] || undefined,
                stage: req.headers["x-stage"] || "dev",
                functionName: req.headers["x-function"] || "handler",
            };
            Promise.resolve(handler(event, context))
                .then((out) => {
                    res.setHeader("Content-Type", "application/json");
                    res.statusCode = 200;
                    res.end(JSON.stringify(out));
                })
                .catch((err) => {
                    res.setHeader("Content-Type", "application/json");
                    res.statusCode = 500;
                    res.end(JSON.stringify({ error: String(err && err.message || err) }));
                });
        });
    };
}

/**
 * Creates a raw handler from a module path (e.g. "src/handler.handler").
 * Loads the module and returns the handler for use with adapters.
 * @param {string} modulePath - Path like "src/handler.handler" (file path + export name)
 * @returns {Promise<RawHandler>}
 */
async function loadHandler(modulePath) {
    const lastDot = modulePath.lastIndexOf(".");
    const filePath = lastDot > 0 ? modulePath.slice(0, lastDot) : modulePath;
    const exportName = lastDot > 0 ? modulePath.slice(lastDot + 1) : "handler";
    const path = require("path");
    const resolved = path.isAbsolute(filePath) ? filePath : path.resolve(process.cwd(), filePath);
    const mod = await import(resolved.startsWith("file:") ? resolved : "file://" + resolved);
    const fn = mod[exportName] || mod.default;
    if (typeof fn !== "function") throw new Error(`Handler not found: ${modulePath}`);
    return fn;
}

module.exports = { createHttpHandler, loadHandler };
