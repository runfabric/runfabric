import type { ProjectConfig } from "@runfabric/core";
import { isRecord, readOptionalBooleanAtPath } from "./shared";

export function readDeployConfigAtPath(
  value: unknown,
  path: string,
  errors: string[]
): ProjectConfig["deploy"] | undefined {
  if (value === undefined) {
    return undefined;
  }
  if (!isRecord(value)) {
    errors.push(`${path} must be an object`);
    return undefined;
  }

  const rollbackOnFailure = readOptionalBooleanAtPath(
    value.rollbackOnFailure,
    `${path}.rollbackOnFailure`,
    errors
  );

  if (rollbackOnFailure === undefined) {
    return undefined;
  }

  return { rollbackOnFailure };
}
