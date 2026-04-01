"use strict";

/**
 * Local Sentry addon scaffold implementing the RunFabric Addon contract.
 * This module is intentionally minimal and safe: it injects env/config hints and
 * generates a helper file, but does not auto-patch handlers.
 */
module.exports = {
  name: "sentry",
  kind: "addon",
  version: "0.1.0",

  supports(input) {
    const runtime = String(input?.runtime || "").toLowerCase();
    const provider = String(input?.provider || "").toLowerCase();

    // Start conservative: support nodejs on common serverless provider ids.
    if (runtime !== "nodejs") return false;
    return provider === "aws" || provider === "aws-lambda" || provider === "";
  },

  async apply(input) {
    const addonConfig = input?.addonConfig || {};
    const options = addonConfig.options || {};
    const secrets = addonConfig.secrets || {};

    const env = {};
    const warnings = [];

    // SENTRY_DSN is typically injected by Go engine secret resolution.
    if (!secrets.SENTRY_DSN) {
      warnings.push(
        "sentry addon: addons.sentry.secrets.SENTRY_DSN is not configured; runtime init will be inert"
      );
    }

    if (typeof options.tracesSampleRate === "number") {
      env.SENTRY_TRACES_SAMPLE_RATE = String(options.tracesSampleRate);
    }
    if (typeof options.environment === "string" && options.environment.trim() !== "") {
      env.SENTRY_ENVIRONMENT = options.environment.trim();
    }
    if (typeof options.release === "string" && options.release.trim() !== "") {
      env.SENTRY_RELEASE = options.release.trim();
    }

    // Generate a helper the app can import explicitly.
    const files = [
      {
        path: ".runfabric/generated/sentry/init-sentry.cjs",
        content: [
          "\"use strict\";",
          "",
          "function initSentry(Sentry) {",
          "  if (!Sentry || typeof Sentry.init !== \"function\") return;",
          "  const dsn = process.env.SENTRY_DSN;",
          "  if (!dsn) return;",
          "  const tracesSampleRate = Number(process.env.SENTRY_TRACES_SAMPLE_RATE || \"0\");",
          "  const environment = process.env.SENTRY_ENVIRONMENT || undefined;",
          "  const release = process.env.SENTRY_RELEASE || undefined;",
          "  Sentry.init({ dsn, tracesSampleRate, environment, release });",
          "}",
          "",
          "module.exports = { initSentry };",
          ""
        ].join("\n")
      }
    ];

    warnings.push(
      "sentry addon: generated .runfabric/generated/sentry/init-sentry.cjs; import and call initSentry(Sentry) in your app entrypoint"
    );

    return {
      env,
      files,
      warnings
    };
  }
};
