import { normalizeRuntimeFamily, runtimeFamilyList, type RuntimeFamily } from "@runfabric/core";

function parseRuntimeValue(value: string, path: string, errors: string[]): RuntimeFamily | undefined {
  const normalized = normalizeRuntimeFamily(value);
  if (!normalized) {
    errors.push(`${path} must be one of: ${runtimeFamilyList()}`);
    return undefined;
  }
  return normalized;
}

export function readRequiredRuntimeAtPath(
  value: string | undefined,
  path: string,
  errors: string[]
): RuntimeFamily | undefined {
  if (!value || value.trim().length === 0) {
    return undefined;
  }
  return parseRuntimeValue(value, path, errors);
}

export function readOptionalRuntimeAtPath(
  value: unknown,
  path: string,
  errors: string[]
): RuntimeFamily | undefined {
  if (value === undefined) {
    return undefined;
  }
  if (typeof value !== "string") {
    errors.push(`${path} must be a string`);
    return undefined;
  }
  if (value.trim().length === 0) {
    errors.push(`${path} must be a non-empty string`);
    return undefined;
  }
  return parseRuntimeValue(value, path, errors);
}
