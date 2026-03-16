const { createHttpHandler } = require("../raw");

function nestHandler(handler) {
    return createHttpHandler(handler);
}

function nestMiddleware(handler) {
    const fn = createHttpHandler(handler);
    return (req, res, next) => {
        if (res.headersSent) return next();
        fn(req, res);
    };
}

module.exports = { nestHandler, nestMiddleware };
