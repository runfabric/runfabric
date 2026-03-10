import { spawnSync } from "node:child_process";

const publishOrder = [
  "@runfabric/core",
  "@runfabric/planner",
  "@runfabric/builder",
  "@runfabric/runtime-node",
  "@runfabric/provider-aws-lambda",
  "@runfabric/provider-gcp-functions",
  "@runfabric/provider-azure-functions",
  "@runfabric/provider-cloudflare-workers",
  "@runfabric/provider-vercel",
  "@runfabric/provider-netlify",
  "@runfabric/provider-alibaba-fc",
  "@runfabric/provider-digitalocean-functions",
  "@runfabric/provider-fly-machines",
  "@runfabric/provider-ibm-openwhisk",
  "@runfabric/cli"
];

const dryRun = process.argv.includes("--dry-run");

if (!dryRun && (!process.env.NPM_TOKEN || process.env.NPM_TOKEN.trim().length === 0)) {
  console.error("NPM_TOKEN is required for publish");
  process.exit(1);
}

for (const pkg of publishOrder) {
  const args = ["--filter", pkg, "publish", "--access", "public", "--no-git-checks"];
  if (dryRun) {
    args.push("--dry-run");
  }

  console.log(`publishing ${pkg}${dryRun ? " (dry-run)" : ""}`);
  const result = spawnSync("pnpm", args, {
    stdio: "inherit",
    env: {
      ...process.env
    }
  });

  if (result.status !== 0) {
    process.exit(result.status ?? 1);
  }
}

console.log("publish sequence complete");
