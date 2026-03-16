const { createHttpHandler } = require("../raw");

function mount(app, handler, path = "/", method = "post") {
    const fn = createHttpHandler(handler);
    const m = method.toLowerCase();
    if (typeof app[m] === "function") {
        app[m](path, (req, res) => fn(req, res));
    } else {
        app.use(path, (req, res, next) => {
            if (req.method.toLowerCase() !== m) return next();
            fn(req, res);
        });
    }
}

function createRouter(routes) {
    let router;
    try {
        router = require("express").Router();
    } catch (e) {
        throw new Error("Express is required: npm install express");
    }
    for (const { path = "/", method = "post", handler } of routes) {
        mount(router, handler, path, method);
    }
    return router;
}

module.exports = { mount, createRouter };
