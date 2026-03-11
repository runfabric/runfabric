import { spawnSync } from "node:child_process";

const tests = [
  "tests/provider-real-deploy-contracts.test.ts",
  "tests/provider-endpoints.test.ts",
  "tests/provider-adapters.integration.test.ts",
  "tests/aws-provider-config-wiring.test.ts",
  "tests/provider-observability-contracts.test.ts"
];

const result = spawnSync(
  "tsx",
  ["--tsconfig", "tsconfig.runtime.json", "--test", ...tests],
  {
    cwd: process.cwd(),
    stdio: "inherit",
    env: process.env
  }
);

process.exit(result.status ?? 1);
