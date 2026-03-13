import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import {
  ARTIFACT_MANIFEST_SCHEMA_VERSION,
  ENGINE_CONTRACT_ABI_VERSION,
  ENGINE_CONTRACT_API_VERSION,
  ENGINE_CONTRACT_COMPATIBILITY_POLICY,
  RUNTIME_FAMILIES,
  RUNTIME_MODES,
  validateArtifactManifest
} from "../packages/core/src/index.ts";

interface ManifestFixture {
  name: string;
  manifest: unknown;
}

interface ManifestFixtureMatrix {
  schemaVersion: number;
  description: string;
  fixtures: ManifestFixture[];
}

const repoRoot = fileURLToPath(new URL("..", import.meta.url));
const fixturePath = join(repoRoot, "tests", "fixtures", "artifact-manifest-v2", "matrix.json");

function clone<T>(value: T): T {
  return JSON.parse(JSON.stringify(value)) as T;
}

async function loadFixtureMatrix(): Promise<ManifestFixtureMatrix> {
  const raw = await readFile(fixturePath, "utf8");
  return JSON.parse(raw) as ManifestFixtureMatrix;
}

test("artifact manifest v2 fixtures cover every runtime family and mode", async () => {
  const matrix = await loadFixtureMatrix();
  assert.equal(matrix.schemaVersion, 1);
  assert.equal(matrix.fixtures.length, RUNTIME_FAMILIES.length * RUNTIME_MODES.length);

  const seen = new Set<string>();
  for (const fixture of matrix.fixtures) {
    const errors = validateArtifactManifest(fixture.manifest);
    assert.deepEqual(errors, [], `invalid fixture ${fixture.name}:\n- ${errors.join("\n- ")}`);

    const manifest = fixture.manifest as Record<string, unknown>;
    seen.add(`${manifest.runtimeFamily}:${manifest.runtimeMode}`);
  }

  for (const family of RUNTIME_FAMILIES) {
    for (const mode of RUNTIME_MODES) {
      assert.equal(seen.has(`${family}:${mode}`), true, `missing fixture for ${family}/${mode}`);
    }
  }
});

test("artifact manifest v2 rejects schema downgrade and upgrade", async () => {
  const matrix = await loadFixtureMatrix();
  const base = matrix.fixtures[0]?.manifest;
  assert.ok(base, "at least one fixture is required");

  const downgrade = clone(base) as Record<string, unknown>;
  downgrade.schemaVersion = ARTIFACT_MANIFEST_SCHEMA_VERSION - 1;
  downgrade.build = {
    ...(downgrade.build as Record<string, unknown>),
    manifestVersion: ARTIFACT_MANIFEST_SCHEMA_VERSION - 1
  };

  const downgradeErrors = validateArtifactManifest(downgrade);
  assert.equal(
    downgradeErrors.some((error) => error.includes("older than required")),
    true,
    `expected downgrade schema rejection, got: ${downgradeErrors.join(" | ")}`
  );
  assert.equal(
    downgradeErrors.some((error) => error.includes("build.manifestVersion must be")),
    true,
    `expected downgrade build.manifestVersion rejection, got: ${downgradeErrors.join(" | ")}`
  );

  const upgrade = clone(base) as Record<string, unknown>;
  upgrade.schemaVersion = ARTIFACT_MANIFEST_SCHEMA_VERSION + 1;
  upgrade.build = {
    ...(upgrade.build as Record<string, unknown>),
    manifestVersion: ARTIFACT_MANIFEST_SCHEMA_VERSION + 1
  };

  const upgradeErrors = validateArtifactManifest(upgrade);
  assert.equal(
    upgradeErrors.some((error) => error.includes("newer than supported")),
    true,
    `expected upgrade schema rejection, got: ${upgradeErrors.join(" | ")}`
  );
  assert.equal(
    upgradeErrors.some((error) => error.includes("build.manifestVersion must be")),
    true,
    `expected upgrade build.manifestVersion rejection, got: ${upgradeErrors.join(" | ")}`
  );
});

test("artifact manifest v2 enforces engine contract version constants", async () => {
  const matrix = await loadFixtureMatrix();
  const base = matrix.fixtures[0]?.manifest;
  assert.ok(base, "at least one fixture is required");

  const incompatible = clone(base) as Record<string, unknown>;
  incompatible.engineContract = {
    apiVersion: "2.1.0",
    abiVersion: "3.0.0",
    compatibilityPolicy: "strict"
  };

  const errors = validateArtifactManifest(incompatible);
  assert.equal(errors.some((error) => error.includes(`apiVersion must be ${ENGINE_CONTRACT_API_VERSION}`)), true);
  assert.equal(errors.some((error) => error.includes(`abiVersion must be ${ENGINE_CONTRACT_ABI_VERSION}`)), true);
  assert.equal(
    errors.some((error) => error.includes(`compatibilityPolicy must be ${ENGINE_CONTRACT_COMPATIBILITY_POLICY}`)),
    true
  );
});
