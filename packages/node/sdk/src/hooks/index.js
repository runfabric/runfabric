/**
 * Lifecycle hook loading and execution.
 * Load hook modules from paths and run beforeBuild / afterBuild / beforeDeploy / afterDeploy.
 * @module @runfabric/sdk/hooks
 */

const path = require("path");
const { pathToFileURL } = require("url");

const PHASES = ["beforeBuild", "afterBuild", "beforeDeploy", "afterDeploy"];

/**
 * Load hook modules from an array of paths (e.g. from runfabric.yml hooks).
 * Each path is resolved relative to cwd. Supports .js and .mjs (ESM via dynamic import).
 * @param {string[]} hookPaths - Paths like ["./hooks.mjs", "./other-hooks.js"]
 * @param {string} [cwd=process.cwd()] - Base directory for relative paths
 * @returns {Promise<Array<{ name?: string, beforeBuild?: Function, afterBuild?: Function, beforeDeploy?: Function, afterDeploy?: Function }>>}
 */
async function loadHookModules(hookPaths, cwd = process.cwd()) {
    if (!Array.isArray(hookPaths) || hookPaths.length === 0) {
        return [];
    }

    const modules = [];
    for (const hookPath of hookPaths) {
        const resolved = path.isAbsolute(hookPath) ? hookPath : path.resolve(cwd, hookPath);
        const url = resolved.startsWith("file:") ? resolved : pathToFileURL(resolved).href;
        const mod = await import(url);
        const hook = mod.default != null ? mod.default : mod;
        if (hook && typeof hook === "object") {
            modules.push(hook);
        }
    }
    return modules;
}

/**
 * Run a lifecycle phase across loaded hook modules. Calls phase(context) on each hook that has that method.
 * @param {Array<{ beforeBuild?: Function, afterBuild?: Function, beforeDeploy?: Function, afterDeploy?: Function }>} hookModules - Loaded hook objects from loadHookModules
 * @param {"beforeBuild"|"afterBuild"|"beforeDeploy"|"afterDeploy"} phase - Phase to run
 * @param {object} context - Context passed to the hook (e.g. { cwd, config } for build, { cwd, config, deployments } for deploy)
 * @returns {Promise<void>}
 */
async function runLifecycleHooks(hookModules, phase, context) {
    if (!PHASES.includes(phase)) {
        throw new Error(`Invalid lifecycle phase: ${phase}. Must be one of ${PHASES.join(", ")}`);
    }
    if (!Array.isArray(hookModules)) return;

    for (const hook of hookModules) {
        const fn = hook[phase];
        if (typeof fn === "function") {
            await Promise.resolve(fn.call(hook, context));
        }
    }
}

module.exports = {
    loadHookModules,
    runLifecycleHooks,
    PHASES,
};
