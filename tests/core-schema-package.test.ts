import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { existsSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

const repoRoot = fileURLToPath(new URL("..", import.meta.url));

test("@runfabric/core publishes runfabric.schema.json", async () => {
  const packageJsonPath = join(repoRoot, "packages", "core", "package.json");
  const packageJson = JSON.parse(await readFile(packageJsonPath, "utf8"));
  assert.equal(Array.isArray(packageJson.files), true);
  assert.equal(packageJson.files.includes("runfabric.schema.json"), true);

  const schemaPath = join(repoRoot, "packages", "core", "runfabric.schema.json");
  assert.equal(existsSync(schemaPath), true);
  const schema = JSON.parse(await readFile(schemaPath, "utf8"));
  assert.equal(schema.$id, "https://runfabric.dev/schema/runfabric.schema.json");
  assert.equal(schema.properties?.service?.type, "string");
});
