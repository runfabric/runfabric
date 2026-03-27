/**
 * Single handler function that manages internally: (event, context) => response or (request) => Promise<UniversalResponse>.
 * Accepts a raw handler function or an Express/Fastify/Nest app. Apps use the universal contract (request → response) via real HTTP or inject.
 * @module @runfabric/sdk/adapters/handler
 */

const { createHttpHandler } = require("./raw");
const expressAdapter = require("./express");
const fastifyAdapter = require("./fastify");
const nestAdapter = require("./nest");
const universal = require("./universalHandler");

/**
 * RunFabric universal handler: (request: UniversalRequest) => Promise<UniversalResponse>, or raw (event, context) => response.
 * Also callable as (req, res) for HTTP and has .mountExpress(), .mountFastify(), .forNest().
 * @typedef {(event: object, context?: object) => Promise<object> | object} UniversalHandler
 */

/**
 * Creates a universal handler from a raw function or from an Express/Fastify/Nest app.
 * - createHandler((event, context) => response) — raw function; invoked directly; has .mountExpress(), .mountFastify(), .forNest().
 * - createHandler(expressApp | fastifyInstance | nestApp) — universal contract (request → { status, headers, body }); Express via real server + fetch, Fastify via inject.
 * @param {((event: object, context?: object) => Promise<object> | object) | import('express').Application | import('fastify').FastifyInstance | import('@nestjs/common').INestApplication} handlerOrApp
 * @returns {UniversalHandler & { mountExpress: Function, mountFastify: Function, forNest: Function }}
 */
function createHandler(handlerOrApp) {
    let handler;
    if (universal.isNestApplication(handlerOrApp) || universal.isFastifyInstance(handlerOrApp) || universal.isExpressApplication(handlerOrApp)) {
        const universalHandler = universal.createHandler(handlerOrApp);
        handler = (event, context) => universalHandler(event || {});
    } else if (typeof handlerOrApp === "function") {
        handler = handlerOrApp;
    } else {
        throw new Error("createHandler expects a function (event, context) => response or an Express/Fastify/Nest app");
    }

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

    const fn = (req, res) => httpHandler(req, res);
    fn.mountExpress = mountExpress;
    fn.mountFastify = mountFastify;
    fn.forNest = forNest;
    return fn;
}

module.exports = { createHandler };
