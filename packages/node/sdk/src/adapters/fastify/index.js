const { createHttpHandler } = require("../raw");

function register(fastify, handler, options = {}) {
    const { url = "/", method = "POST" } = options;
    const fn = createHttpHandler(handler);
    fastify.route({
        method: method.toUpperCase(),
        url,
        handler: (request, reply) => {
            fn(request.raw, reply.raw);
        },
    });
}

function runfabricFastifyPlugin(fastify, _opts, done) {
    fastify.decorate("runfabricMount", (handler, options) => register(fastify, handler, options));
    done();
}

module.exports = { register, runfabricFastifyPlugin };
