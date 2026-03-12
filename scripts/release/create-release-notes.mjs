import { existsSync, mkdirSync, writeFileSync } from "node:fs";
import { resolve } from "node:path";

function readArg(name) {
  const index = process.argv.indexOf(name);
  if (index === -1) {
    return undefined;
  }
  return process.argv[index + 1];
}

function buildTemplate(version) {
  return [
    `# runfabric \`${version}\``,
    "",
    "## Highlights",
    "",
    "- Add key feature highlights for this release.",
    "- Add notable CLI/config/runtime changes.",
    "",
    "## Breaking Changes",
    "",
    "- None.",
    "",
    "## Notes",
    "",
    "- Add migration notes, caveats, and rollout guidance.",
    ""
  ].join("\n");
}

const version = readArg("--version") || process.env.RELEASE_VERSION;
if (!version || version.trim().length === 0) {
  console.error("version is required (--version or RELEASE_VERSION)");
  process.exit(1);
}

const force = process.argv.includes("--force");
const notesDir = resolve("release-notes");
const notesPath = resolve(notesDir, `${version}.md`);

if (existsSync(notesPath) && !force) {
  console.error(`release notes already exist: ${notesPath}`);
  console.error("use --force to overwrite");
  process.exit(1);
}

mkdirSync(notesDir, { recursive: true });
writeFileSync(notesPath, buildTemplate(version), "utf8");
console.log(`wrote release notes: ${notesPath}`);
