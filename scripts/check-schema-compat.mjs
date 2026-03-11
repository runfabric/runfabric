import { readdirSync, statSync } from "node:fs";
import { join, relative } from "node:path";
import { spawnSync } from "node:child_process";

const repoRoot = process.cwd();
const examplesRoot = join(repoRoot, "examples");
const cliEntry = join(repoRoot, "apps", "cli", "src", "index.ts");
const runtimeTsConfig = join(repoRoot, "tsconfig.runtime.json");
const tsxBin = join(
  repoRoot,
  "node_modules",
  ".bin",
  process.platform === "win32" ? "tsx.cmd" : "tsx"
);

function collectRunfabricConfigs(dir, output) {
  for (const entry of readdirSync(dir)) {
    const fullPath = join(dir, entry);
    const stat = statSync(fullPath);
    if (stat.isDirectory()) {
      collectRunfabricConfigs(fullPath, output);
      continue;
    }
    if (!entry.endsWith(".yml")) {
      continue;
    }
    if (!entry.startsWith("runfabric")) {
      continue;
    }
    if (entry.includes(".compose.")) {
      continue;
    }
    output.push(fullPath);
  }
}

const configs = [];
collectRunfabricConfigs(examplesRoot, configs);

if (configs.length === 0) {
  console.error("no runfabric example config files found");
  process.exit(1);
}

for (const configPath of configs) {
  const relPath = relative(repoRoot, configPath);
  const result = spawnSync(
    tsxBin,
    ["--tsconfig", runtimeTsConfig, cliEntry, "plan", "-c", relPath, "--json"],
    {
      cwd: repoRoot,
      encoding: "utf8"
    }
  );

  if (result.status !== 0) {
    console.error(`schema compatibility failed for ${relPath}`);
    if (result.stderr) {
      console.error(result.stderr.trim());
    }
    if (result.stdout) {
      console.error(result.stdout.trim());
    }
    process.exit(result.status ?? 1);
  }

  const stdout = result.stdout.trim();
  if (!stdout) {
    console.error(`schema compatibility check returned empty output for ${relPath}`);
    process.exit(1);
  }
}

console.log(`schema compatibility OK for ${configs.length} example config(s)`);
