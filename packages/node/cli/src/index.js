const { spawnSync } = require("node:child_process");
const { resolveBinary } = require("./bin");

function run(command, args = []) {
    const bin = resolveBinary();
    const res = spawnSync(bin, [command, ...args, "--json"], {
        encoding: "utf-8",
    });

    if (res.error) throw res.error;
    if (!res.stdout) throw new Error(res.stderr || "empty stdout");

    const parsed = JSON.parse(res.stdout);
    if (!parsed.ok) {
        throw new Error(parsed.error?.message || "runfabric command failed");
    }
    return parsed.result;
}

function deploy(stage = "dev", configPath = "runfabric.yml", options = {}) {
    const args = ["--stage", stage, "--config", configPath];
    if (options.rollbackOnFailure === false) args.push("--no-rollback-on-failure");
    return run("deploy", args);
}

function inspect(stage = "dev", configPath = "runfabric.yml") {
    return run("inspect", ["--stage", stage, "--config", configPath]);
}

function build(stage = "dev", configPath = "runfabric.yml", outDir = "") {
    const args = ["--stage", stage, "--config", configPath];
    if (outDir) args.push("--out", outDir);
    return run("build", args);
}

module.exports = {
    deploy,
    inspect,
    build,
    run,
};
