/**
 * RunFabric Node SDK — handler contract, HTTP adapter, framework adapters, lifecycle hooks.
 * For the CLI and programmatic deploy/inspect/build, use @runfabric/cli.
 */
const adapters = require("./adapters");
const { createHttpHandler } = require("./http/index");
const { loadHandler } = require("./adapters/raw");
const hooks = require("./hooks");

module.exports = {
    ...adapters,
    createHttpHandler,
    loadHandler,
    ...hooks,
};
