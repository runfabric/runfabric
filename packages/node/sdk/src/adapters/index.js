/**
 * RunFabric Node adapters: single handler or Express, Fastify, Nest, raw.
 * Prefer createHandler(handlerOrApp): pass a function (event, context) => response or an Express/Fastify/Nest app.
 */
const raw = require("./raw");
const express = require("./express");
const fastify = require("./fastify");
const nest = require("./nest");
const { createHandler } = require("./handler");
const universalHandler = require("./universalHandler");

module.exports = {
    createHandler,
    raw,
    express,
    fastify,
    nest,
    universalHandler,
};
