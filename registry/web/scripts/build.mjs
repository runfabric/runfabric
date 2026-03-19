import crypto from "node:crypto";
import fs from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { buildDocsIndex } from "../lib/docs/loader.mjs";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const webRoot = path.resolve(__dirname, "..");
const appDir = path.join(webRoot, "app");
const distDir = path.join(webRoot, "dist");
const assetsDir = path.join(distDir, "assets");

function hashName(content) {
  return crypto.createHash("sha256").update(content).digest("hex").slice(0, 12);
}

async function ensureCleanDist() {
  await fs.rm(distDir, { recursive: true, force: true });
  await fs.mkdir(assetsDir, { recursive: true });
}

async function read(filePath) {
  return fs.readFile(filePath, "utf8");
}

async function write(filePath, content) {
  await fs.mkdir(path.dirname(filePath), { recursive: true });
  await fs.writeFile(filePath, content, "utf8");
}

function inlineRegistryClient(mainSource, clientSource) {
  const inlinedClient = clientSource.replace("export function createRegistryClient", "function createRegistryClient");
  return mainSource.replace("/*__REGISTRY_CLIENT__*/", inlinedClient);
}

async function buildAssets() {
  const [template, css, mainSource, clientSource] = await Promise.all([
    read(path.join(appDir, "index.template.html")),
    read(path.join(appDir, "styles.css")),
    read(path.join(appDir, "main.js")),
    read(path.join(webRoot, "lib", "registry", "client.js")),
  ]);

  const appJS = inlineRegistryClient(mainSource, clientSource);
  const jsName = `assets/app.${hashName(appJS)}.js`;
  const cssName = `assets/app.${hashName(css)}.css`;

  await write(path.join(distDir, jsName), appJS);
  await write(path.join(distDir, cssName), css);

  const html = template.replaceAll("__APP_JS__", jsName).replaceAll("__APP_CSS__", cssName);
  await write(path.join(distDir, "index.html"), html);
}

async function buildDocsArtifact() {
  const docsIndex = await buildDocsIndex({ webDir: webRoot });
  const payload = `${JSON.stringify(docsIndex, null, 2)}\n`;
  await write(path.join(distDir, "docs-index.json"), payload);
  await write(path.join(webRoot, "content", "generated", "docs-index.json"), payload);
}

async function main() {
  await ensureCleanDist();
  await Promise.all([buildAssets(), buildDocsArtifact()]);
  process.stdout.write(`registry web build completed: ${distDir}\n`);
}

main().catch((error) => {
  process.stderr.write(`registry web build failed: ${error instanceof Error ? error.stack || error.message : String(error)}\n`);
  process.exitCode = 1;
});
