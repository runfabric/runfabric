/**
 * Single handler function that manages internally: one (event, context) => response
 * can be used as raw HTTP, or mounted on Express, Fastify, or Nest via the same object.
 * @module @runfabric/sdk/adapters/handler
 */

const { createHttpHandler } = require("./raw");
const expressAdapter = require("./express");   // express/index.js
const fastifyAdapter = require("./fastify");   // fastify/index.js
const nestAdapter = require("./nest");         // nest/index.js

/**
 * Creates a single handler that manages internally: use as (req, res), or call
 * .mountExpress(), .mountFastify(), or .forNest() to plug into a framework.
 * @param {(event: object, context: { requestId?: string, stage?: string, functionName?: string }) => Promise<object> | object} handler - Your (event, context) => response function
 * @returns {HttpHandler & { mountExpress: Function, mountFastify: Function, forNest: Function }}
 */
function createHandler(handler) {
    const httpHandler = createHttpHandler(handler);

    function mountExpress(app, path = "/", method = "post") {
        expressAdapter.mount(app, handler, path, method);
    }

    function mountFastify(fastifyInstance, options = {}) {
        fastifyAdapter.register(fastifyInstance, handler, options);
    }

    function forNest() {
        return nestAdapter.nestHandler(handler);
    }

    // Same object is callable as (req, res) and has mount methods
    const fn = (req, res) => httpHandler(req, res);
    fn.mountExpress = mountExpress;
    fn.mountFastify = mountFastify;
    fn.forNest = forNest;
    return fn;
}

module.exports = { createHandler };
