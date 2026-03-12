import { spawnSync } from "node:child_process";
import { mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { publishOrder } from "./publish-order.mjs";

function readArgValue(name) {
  const index = process.argv.indexOf(name);
  if (index === -1) {
    return undefined;
  }

  const value = process.argv[index + 1];
  if (!value || value.startsWith("--")) {
    console.error(`${name} requires a value`);
    process.exit(1);
  }

  return value;
}

const dryRun = process.argv.includes("--dry-run");
const distTag = readArgValue("--tag") ?? process.env.NPM_DIST_TAG ?? "latest";
const otp = readArgValue("--otp") ?? process.env.NPM_OTP;
const npmToken = process.env.NPM_TOKEN?.trim() || "";

if (!dryRun && npmToken.length === 0) {
  console.error("NPM_TOKEN is required for publish");
  process.exit(1);
}

let tempNpmrcDir;
let tempNpmrcPath;

if (npmToken.length > 0) {
  tempNpmrcDir = mkdtempSync(join(tmpdir(), "runfabric-publish-"));
  tempNpmrcPath = join(tempNpmrcDir, ".npmrc");
  writeFileSync(
    tempNpmrcPath,
    [
      "registry=https://registry.npmjs.org/",
      `//registry.npmjs.org/:_authToken=${npmToken}`,
      ""
    ].join("\n"),
    "utf8"
  );
}

try {
  for (const pkg of publishOrder) {
    const args = ["--filter", pkg, "publish", "--access", "public", "--tag", distTag, "--no-git-checks"];
    if (otp && otp.trim().length > 0) {
      args.push("--otp", otp.trim());
    }
    if (dryRun) {
      args.push("--dry-run");
    }

    console.log(`publishing ${pkg} with tag ${distTag}${dryRun ? " (dry-run)" : ""}`);
    const result = spawnSync("pnpm", args, {
      stdio: "inherit",
      env: {
        ...process.env,
        ...(tempNpmrcPath ? { NPM_CONFIG_USERCONFIG: tempNpmrcPath, NODE_AUTH_TOKEN: npmToken } : {})
      }
    });

    if (result.status !== 0) {
      console.error("");
      console.error("publish failed");
      console.error("if npm returns E403 requiring 2FA:");
      console.error("1) use a granular npm token with publish permission and bypass-2FA enabled, or");
      console.error("2) provide one-time OTP via NPM_OTP or --otp <code> for local publish");
      process.exit(result.status ?? 1);
    }
  }
} finally {
  if (tempNpmrcDir) {
    rmSync(tempNpmrcDir, { recursive: true, force: true });
  }
}

console.log("publish sequence complete");
