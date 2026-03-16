/**
 * RunFabric Node adapters: single handler or Express, Fastify, Nest, raw.
 * Prefer createHandler(handler): one function that manages internally and can mount on any framework.
 */
const raw = require("./raw");
const express = require("./express");   // adapters/express/index.js
const fastify = require("./fastify");   // adapters/fastify/index.js
const nest = require("./nest");         // adapters/nest/index.js
const { createHandler } = require("./handler");

module.exports = {
    createHandler,
    raw,
    express,
    fastify,
    nest,
};
