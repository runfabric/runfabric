import { createHmac } from "node:crypto";
import { readFileSync, writeFileSync, existsSync } from "node:fs";
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

const key = process.env.RELEASE_NOTES_SIGNING_KEY;
if (!key || key.trim().length === 0) {
  console.error("RELEASE_NOTES_SIGNING_KEY is required");
  process.exit(1);
}

const notesPath = resolve("release-notes", `${version}.md`);
if (!existsSync(notesPath)) {
  console.error(`missing release notes file: ${notesPath}`);
  process.exit(1);
}

const notesContent = readFileSync(notesPath, "utf8");
const signature = createHmac("sha256", key).update(notesContent, "utf8").digest("hex");
const signaturePath = `${notesPath}.sig`;
writeFileSync(signaturePath, `${signature}\n`, "utf8");
console.log(`wrote signature: ${signaturePath}`);
