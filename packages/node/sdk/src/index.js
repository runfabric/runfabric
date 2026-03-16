/**
 * RunFabric Node SDK — handler contract, HTTP adapter, framework adapters.
 * For the CLI and programmatic deploy/inspect/build, use @runfabric/cli.
 */
const adapters = require("./adapters");
const { createHttpHandler } = require("./http/index");
const { loadHandler } = require("./adapters/raw");

module.exports = {
    ...adapters,
    createHttpHandler,
    loadHandler,
};
