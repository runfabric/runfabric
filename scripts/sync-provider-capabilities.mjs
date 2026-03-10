import { readFileSync, writeFileSync } from "node:fs";
import { resolve } from "node:path";

const providerToCapabilitiesFile = {
  "aws-lambda": "packages/provider-aws-lambda/src/capabilities.ts",
  "gcp-functions": "packages/provider-gcp-functions/src/capabilities.ts",
  "azure-functions": "packages/provider-azure-functions/src/capabilities.ts",
  "cloudflare-workers": "packages/provider-cloudflare-workers/src/capabilities.ts",
  vercel: "packages/provider-vercel/src/capabilities.ts",
  netlify: "packages/provider-netlify/src/capabilities.ts",
  "alibaba-fc": "packages/provider-alibaba-fc/src/capabilities.ts",
  "digitalocean-functions": "packages/provider-digitalocean-functions/src/capabilities.ts",
  "fly-machines": "packages/provider-fly-machines/src/capabilities.ts",
  "ibm-openwhisk": "packages/provider-ibm-openwhisk/src/capabilities.ts"
};

const matrixPath = resolve("packages/planner/src/capability-matrix.ts");

function extractObjectLiteral(content, anchor) {
  const anchorIndex = content.search(anchor);
  if (anchorIndex < 0) {
    throw new Error(`Could not find anchor ${anchor}`);
  }

  const objectStart = content.indexOf("{", anchorIndex);
  if (objectStart < 0) {
    throw new Error("Could not find object start");
  }

  let depth = 0;
  let objectEnd = -1;
  for (let index = objectStart; index < content.length; index += 1) {
    const char = content[index];
    if (char === "{") {
      depth += 1;
    } else if (char === "}") {
      depth -= 1;
      if (depth === 0) {
        objectEnd = index;
        break;
      }
    }
  }

  if (objectEnd < 0) {
    throw new Error("Could not find object end");
  }

  return content.slice(objectStart, objectEnd + 1);
}

function parseTsObjectLiteral(objectLiteral) {
  const normalized = objectLiteral
    .replace(/\/\*[\s\S]*?\*\//g, "")
    .replace(/(^|\n)\s*\/\/.*$/g, "$1")
    .replace(/([{,]\s*)([A-Za-z_][A-Za-z0-9_]*)\s*:/g, '$1"$2":')
    .replace(/,\s*([}\]])/g, "$1");

  return JSON.parse(normalized);
}

function stableStringify(value) {
  if (Array.isArray(value)) {
    return `[${value.map((item) => stableStringify(item)).join(",")}]`;
  }
  if (value && typeof value === "object") {
    const entries = Object.entries(value).sort(([a], [b]) =>
      a < b ? -1 : a > b ? 1 : 0
    );
    return `{${entries
      .map(([key, item]) => `${JSON.stringify(key)}:${stableStringify(item)}`)
      .join(",")}}`;
  }
  return JSON.stringify(value);
}

function buildSourceOfTruth() {
  const out = {};
  for (const [provider, relativePath] of Object.entries(providerToCapabilitiesFile)) {
    const content = readFileSync(resolve(relativePath), "utf8");
    const objectLiteral = extractObjectLiteral(content, /ProviderCapabilities\s*=\s*/);
    out[provider] = parseTsObjectLiteral(objectLiteral);
  }
  return out;
}

function parseMatrix() {
  const content = readFileSync(matrixPath, "utf8");
  const objectLiteral = extractObjectLiteral(content, /capabilityMatrix\s*:\s*Record<.*?>\s*=\s*/);
  return parseTsObjectLiteral(objectLiteral);
}

function renderObjectLiteral(value) {
  const json = JSON.stringify(value, null, 2);
  return json.replace(/"([A-Za-z_][A-Za-z0-9_]*)":/g, "$1:");
}

function writeMatrix(sourceOfTruth) {
  const ordered = {};
  for (const provider of Object.keys(providerToCapabilitiesFile)) {
    ordered[provider] = sourceOfTruth[provider];
  }

  const content = [
    'import type { ProviderCapabilities } from "@runfabric/core";',
    "",
    "export const capabilityMatrix: Record<string, ProviderCapabilities> = " +
      renderObjectLiteral(ordered) +
      ";",
    ""
  ].join("\n");

  writeFileSync(matrixPath, content, "utf8");
}

const mode = process.argv.includes("--write") ? "write" : "check";
const sourceOfTruth = buildSourceOfTruth();

if (mode === "write") {
  writeMatrix(sourceOfTruth);
  console.log("capability matrix updated from provider capability files");
  process.exit(0);
}

const matrix = parseMatrix();
const expected = stableStringify(sourceOfTruth);
const actual = stableStringify(matrix);

if (expected !== actual) {
  console.error("capability drift detected between provider capabilities and planner matrix");
  console.error("run: node scripts/sync-provider-capabilities.mjs --write");
  process.exit(1);
}

console.log("capability matrix is in sync");
