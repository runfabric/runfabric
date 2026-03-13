import { RUNTIME_FAMILIES, RUNTIME_MODES } from "./runtime";

export const ARTIFACT_MANIFEST_SCHEMA_VERSION = 2;
export const ENGINE_CONTRACT_API_VERSION = "2.0.0";
export const ENGINE_CONTRACT_ABI_VERSION = "2.0.0";
export const ENGINE_CONTRACT_COMPATIBILITY_POLICY = "semver-minor-forward";

export const ARTIFACT_MANIFEST_FILE_ROLES = [
  "entry-source",
  "runtime-wrapper",
  "runtime-package",
  "manifest"
] as const;

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function readNonEmptyString(value: unknown): string | undefined {
  return typeof value === "string" && value.trim().length > 0 ? value : undefined;
}

function validateGeneratedFileEntry(entry: unknown, index: number, errors: string[]): void {
  if (!isRecord(entry)) {
    errors.push(`files[${index}] must be an object`);
    return;
  }
  if (!readNonEmptyString(entry.path)) {
    errors.push(`files[${index}].path must be a non-empty string`);
  }
  if (typeof entry.bytes !== "number" || Number.isNaN(entry.bytes) || entry.bytes < 0) {
    errors.push(`files[${index}].bytes must be a non-negative number`);
  }
  if (typeof entry.sha256 !== "string" || !/^[a-f0-9]{64}$/i.test(entry.sha256)) {
    errors.push(`files[${index}].sha256 must be a 64-char hex digest`);
  }
  if (
    typeof entry.role !== "string" ||
    !ARTIFACT_MANIFEST_FILE_ROLES.includes(entry.role as (typeof ARTIFACT_MANIFEST_FILE_ROLES)[number])
  ) {
    errors.push(`files[${index}].role must be one of: ${ARTIFACT_MANIFEST_FILE_ROLES.join(", ")}`);
  }
}

export function validateArtifactManifest(value: unknown): string[] {
  const errors: string[] = [];
  if (!isRecord(value)) return ["artifact manifest must be an object"];

  const validateEngineContract = (engineContract: unknown): void => {
    if (!isRecord(engineContract)) {
      errors.push("engineContract must be an object");
      return;
    }
    if (engineContract.apiVersion !== ENGINE_CONTRACT_API_VERSION) errors.push(`engineContract.apiVersion must be ${ENGINE_CONTRACT_API_VERSION}`);
    if (engineContract.abiVersion !== ENGINE_CONTRACT_ABI_VERSION) errors.push(`engineContract.abiVersion must be ${ENGINE_CONTRACT_ABI_VERSION}`);
    if (engineContract.compatibilityPolicy !== ENGINE_CONTRACT_COMPATIBILITY_POLICY) errors.push(`engineContract.compatibilityPolicy must be ${ENGINE_CONTRACT_COMPATIBILITY_POLICY}`);
  };

  const validateBuild = (build: unknown): void => {
    if (!isRecord(build)) {
      errors.push("build must be an object");
      return;
    }
    if (build.manifestVersion !== ARTIFACT_MANIFEST_SCHEMA_VERSION) errors.push(`build.manifestVersion must be ${ARTIFACT_MANIFEST_SCHEMA_VERSION}`);
    if (!readNonEmptyString(build.generatedAt)) errors.push("build.generatedAt must be a non-empty string");
  };

  const schemaVersion = value.schemaVersion;
  if (typeof schemaVersion !== "number" || Number.isNaN(schemaVersion)) errors.push("schemaVersion must be a number");
  else if (schemaVersion > ARTIFACT_MANIFEST_SCHEMA_VERSION) errors.push(`schemaVersion ${schemaVersion} is newer than supported ${ARTIFACT_MANIFEST_SCHEMA_VERSION}`);
  else if (schemaVersion < ARTIFACT_MANIFEST_SCHEMA_VERSION) errors.push(`schemaVersion ${schemaVersion} is older than required ${ARTIFACT_MANIFEST_SCHEMA_VERSION}`);

  if (!readNonEmptyString(value.provider)) errors.push("provider must be a non-empty string");
  if (!readNonEmptyString(value.service)) errors.push("service must be a non-empty string");
  if (
    typeof value.runtimeFamily !== "string" ||
    !RUNTIME_FAMILIES.includes(value.runtimeFamily as (typeof RUNTIME_FAMILIES)[number])
  ) errors.push(`runtimeFamily must be one of: ${RUNTIME_FAMILIES.join(" | ")}`);
  if (
    typeof value.runtimeMode !== "string" ||
    !RUNTIME_MODES.includes(value.runtimeMode as (typeof RUNTIME_MODES)[number])
  ) errors.push(`runtimeMode must be one of: ${RUNTIME_MODES.join(" | ")}`);
  if (!isRecord(value.source) || !readNonEmptyString(value.source.entry)) errors.push("source.entry must be a non-empty string");
  validateEngineContract(value.engineContract);
  validateBuild(value.build);

  if (!Array.isArray(value.files) || value.files.length === 0) errors.push("files must be a non-empty array");
  else value.files.forEach((entry, index) => validateGeneratedFileEntry(entry, index, errors));
  return errors;
}

export function assertArtifactManifest(value: unknown): void {
  const errors = validateArtifactManifest(value);
  if (errors.length > 0) {
    throw new Error(`invalid artifact manifest:\n- ${errors.join("\n- ")}`);
  }
}
