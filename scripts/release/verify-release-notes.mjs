import { createHmac } from "node:crypto";
import { readFileSync, existsSync } from "node:fs";
import { resolve } from "node:path";

function readArg(name) {
  const index = process.argv.indexOf(name);
  if (index === -1) {
    return undefined;
  }
  return process.argv[index + 1];
}

const version = readArg("--version") || process.env.RELEASE_VERSION;
if (!version) {
  console.error("version is required (--version or RELEASE_VERSION)");
  process.exit(1);
}

const changelogPath = resolve("CHANGELOG.md");
const notesPath = resolve("release-notes", `${version}.md`);
const signaturePath = `${notesPath}.sig`;

if (!existsSync(notesPath)) {
  console.error(`missing release notes file: ${notesPath}`);
  process.exit(1);
}
if (!existsSync(signaturePath)) {
  console.error(`missing release notes signature file: ${signaturePath}`);
  process.exit(1);
}

const changelog = readFileSync(changelogPath, "utf8");
if (!changelog.includes(`## [${version}]`)) {
  console.error(`CHANGELOG.md missing section for version ${version}`);
  process.exit(1);
}

const key = process.env.RELEASE_NOTES_SIGNING_KEY;
if (!key || key.trim().length === 0) {
  console.log("RELEASE_NOTES_SIGNING_KEY not set; signature presence check only");
  process.exit(0);
}

const notesContent = readFileSync(notesPath, "utf8");
const expectedSignature = createHmac("sha256", key).update(notesContent, "utf8").digest("hex");
const actualSignature = readFileSync(signaturePath, "utf8").trim();

if (expectedSignature !== actualSignature) {
  console.error("release notes signature verification failed");
  process.exit(1);
}

console.log(`release notes verified for ${version}`);
