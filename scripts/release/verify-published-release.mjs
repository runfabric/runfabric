import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { spawnSync } from "node:child_process";
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

function run(command, args, options = {}) {
  const result = spawnSync(command, args, {
    encoding: "utf8",
    stdio: "pipe",
    ...options
  });
  if (result.status !== 0) {
    const stderr = (result.stderr || "").trim();
    const stdout = (result.stdout || "").trim();
    throw new Error(
      `${command} ${args.join(" ")} failed${stderr ? `: ${stderr}` : stdout ? `: ${stdout}` : ""}`
    );
  }
  return (result.stdout || "").trim();
}

async function verifyGitHubRelease(version, expectedBody) {
  const repo = process.env.GITHUB_REPOSITORY;
  const token = process.env.GITHUB_TOKEN;
  if (!repo || !token) {
    return {
      checked: false
    };
  }

  const response = await fetch(`https://api.github.com/repos/${repo}/releases/tags/v${version}`, {
    headers: {
      Accept: "application/vnd.github+json",
      Authorization: `Bearer ${token}`,
      "User-Agent": "runfabric-release-verifier"
    }
  });

  if (!response.ok) {
    throw new Error(`GitHub release lookup failed: ${response.status} ${response.statusText}`);
  }

  const payload = await response.json();
  const body = typeof payload.body === "string" ? payload.body.trim() : "";
  if (body !== expectedBody.trim()) {
    throw new Error("GitHub release body does not match release-notes/<version>.md");
  }

  return {
    checked: true
  };
}

const version = readArgValue("--version") ?? process.env.RELEASE_VERSION;
if (!version) {
  console.error("--version is required (or set RELEASE_VERSION)");
  process.exit(1);
}

const notesPath = resolve("release-notes", `${version}.md`);
const notesBody = readFileSync(notesPath, "utf8");

for (const pkg of publishOrder) {
  const publishedVersion = run("npm", ["view", `${pkg}@${version}`, "version", "--json"]);
  const normalized = publishedVersion.replace(/^"+|"+$/g, "");
  if (normalized !== version) {
    throw new Error(`expected ${pkg}@${version} but npm view returned ${publishedVersion}`);
  }
}

run("git", ["rev-parse", "--verify", `refs/tags/v${version}`], {
  stdio: "pipe"
});

const releaseCheck = await verifyGitHubRelease(version, notesBody);
if (releaseCheck.checked) {
  console.log(`verified npm publish, git tag, and GitHub release body for v${version}`);
} else {
  console.log(`verified npm publish + git tag for v${version} (GitHub API check skipped)`);
}
