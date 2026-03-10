import { readdirSync, statSync } from "node:fs";
import { join } from "node:path";
import { spawnSync } from "node:child_process";

const roots = ["apps", "packages", "tests"];
const files = [];
const ignoredDirectories = new Set(["node_modules", "dist", ".git"]);

function walk(directory) {
  for (const entry of readdirSync(directory)) {
    if (ignoredDirectories.has(entry)) {
      continue;
    }
    const target = join(directory, entry);
    const stat = statSync(target);
    if (stat.isDirectory()) {
      walk(target);
      continue;
    }
    if (target.endsWith(".ts") && !target.endsWith(".d.ts")) {
      files.push(target);
    }
  }
}

for (const root of roots) {
  walk(root);
}

for (const file of files) {
  const result = spawnSync("node", ["--check", file], { stdio: "inherit" });
  if (result.status !== 0) {
    process.exit(result.status ?? 1);
  }
}

console.log(`checked ${files.length} TypeScript files`);
