import fs from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { docsManifest, docsRootRelative } from "./manifest.mjs";

function firstMatch(pattern, text) {
  const match = text.match(pattern);
  if (!match || !match[1]) {
    return "";
  }
  return String(match[1]).trim();
}

function buildExcerpt(markdown) {
  const lines = String(markdown)
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter((line) => line !== "" && !line.startsWith("#") && !line.startsWith("```"));
  return lines.length > 0 ? lines[0] : "";
}

function toSearchText(markdown) {
  return String(markdown)
    .replace(/```[\s\S]*?```/g, " ")
    .replace(/`[^`]*`/g, " ")
    .replace(/\[[^\]]+\]\([^)]*\)/g, "$1")
    .replace(/[>#*_~|-]/g, " ")
    .replace(/\s+/g, " ")
    .trim();
}

export async function buildDocsIndex(options = {}) {
  const currentFile = fileURLToPath(import.meta.url);
  const webDir = options.webDir ?? path.resolve(path.dirname(currentFile), "..", "..");
  const repoRoot = options.repoRoot ?? path.resolve(webDir, "..", "..", "..");
  const docsRoot = options.docsRoot ?? path.join(repoRoot, docsRootRelative);

  const docs = [];
  for (const entry of docsManifest) {
    const sourcePath = path.join(docsRoot, entry.file);
    const markdown = await fs.readFile(sourcePath, "utf8");
    const headingTitle = firstMatch(/^#\s+(.+)$/m, markdown);
    docs.push({
      slug: entry.slug,
      title: headingTitle || entry.title,
      source: path.posix.join(docsRootRelative, entry.file),
      excerpt: buildExcerpt(markdown),
      markdown,
      searchText: toSearchText(markdown)
    });
  }

  return {
    generatedAt: new Date().toISOString(),
    docs
  };
}
