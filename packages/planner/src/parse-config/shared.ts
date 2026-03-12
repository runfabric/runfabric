export function unquote(value: string): string {
  const trimmed = value.trim();
  if (
    (trimmed.startsWith("\"") && trimmed.endsWith("\"")) ||
    (trimmed.startsWith("'") && trimmed.endsWith("'"))
  ) {
    return trimmed.slice(1, -1);
  }
  return trimmed;
}

export function parseScalar(value: string): unknown {
  const trimmed = value.trim();
  if (trimmed === "true") {
    return true;
  }
  if (trimmed === "false") {
    return false;
  }
  if (trimmed === "null" || trimmed === "~") {
    return null;
  }

  const numeric = Number(trimmed);
  if (!Number.isNaN(numeric) && trimmed !== "") {
    return numeric;
  }
  return unquote(trimmed);
}

function resolveEnvBindingInString(value: string, path: string, errors: string[]): string {
  const envPattern = /\$\{env:([A-Za-z_][A-Za-z0-9_]*)(?:,\s*([^}]*))?\}/g;

  return value.replace(envPattern, (_match, envNameRaw, defaultValueRaw) => {
    const envName = String(envNameRaw || "").trim();
    const envValue = process.env[envName];
    if (envValue !== undefined) {
      return envValue;
    }
    if (typeof defaultValueRaw === "string") {
      return unquote(defaultValueRaw.trim());
    }

    errors.push(`${path} references missing environment variable ${envName}`);
    return "";
  });
}

export function resolveDynamicBindingsAtPath(value: unknown, path: string, errors: string[]): unknown {
  if (typeof value === "string") {
    return resolveEnvBindingInString(value, path, errors);
  }
  if (Array.isArray(value)) {
    return value.map((item, index) => resolveDynamicBindingsAtPath(item, `${path}[${index}]`, errors));
  }
  if (isRecord(value)) {
    const out: Record<string, unknown> = {};
    for (const [key, item] of Object.entries(value)) {
      out[key] = resolveDynamicBindingsAtPath(item, `${path}.${key}`, errors);
    }
    return out;
  }
  return value;
}

export function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

export function isScalar(value: unknown): value is string | number | boolean | null {
  return (
    typeof value === "string" ||
    typeof value === "number" ||
    typeof value === "boolean" ||
    value === null
  );
}

export function isStringArray(value: unknown): value is string[] {
  return Array.isArray(value) && value.every((entry) => typeof entry === "string" && entry.trim().length > 0);
}

export function isFiniteNumber(value: unknown): value is number {
  return typeof value === "number" && Number.isFinite(value);
}

export function readRequiredString(source: Record<string, unknown>, key: string, errors: string[]): string {
  const value = source[key];
  if (typeof value !== "string" || value.trim().length === 0) {
    errors.push(`${key} must be a non-empty string`);
    return "";
  }
  return value.trim();
}

export function readOptionalString(
  source: Record<string, unknown>,
  key: string,
  errors: string[]
): string | undefined {
  if (!(key in source)) {
    return undefined;
  }

  const value = source[key];
  if (typeof value !== "string" || value.trim().length === 0) {
    errors.push(`${key} must be a non-empty string`);
    return undefined;
  }
  return value.trim();
}

export function readStringArrayAtPath(
  value: unknown,
  path: string,
  errors: string[],
  minSize = 0
): string[] {
  if (!Array.isArray(value)) {
    errors.push(`${path} must be an array`);
    return [];
  }

  const values: string[] = [];
  for (let index = 0; index < value.length; index += 1) {
    const item = value[index];
    if (typeof item !== "string" || item.trim().length === 0) {
      errors.push(`${path}[${index}] must be a non-empty string`);
      continue;
    }
    values.push(item.trim());
  }

  if (values.length < minSize) {
    errors.push(`${path} must contain at least ${minSize} value(s)`);
  }
  return values;
}

export function readStringArray(
  source: Record<string, unknown>,
  key: string,
  errors: string[],
  minSize = 0
): string[] {
  return readStringArrayAtPath(source[key], key, errors, minSize);
}

export function readStringRecordAtPath(
  value: unknown,
  path: string,
  errors: string[]
): Record<string, string> | undefined {
  if (value === undefined) {
    return undefined;
  }
  if (!isRecord(value)) {
    errors.push(`${path} must be an object`);
    return undefined;
  }

  const out: Record<string, string> = {};
  for (const [entryKey, entryValue] of Object.entries(value)) {
    if (typeof entryValue !== "string") {
      errors.push(`${path}.${entryKey} must be a string`);
      continue;
    }
    out[entryKey] = entryValue;
  }
  return out;
}

export function readStringRecord(
  source: Record<string, unknown>,
  key: string,
  errors: string[]
): Record<string, string> | undefined {
  return readStringRecordAtPath(source[key], key, errors);
}

export function readOptionalBooleanAtPath(
  value: unknown,
  path: string,
  errors: string[]
): boolean | undefined {
  if (value === undefined) {
    return undefined;
  }
  if (typeof value !== "boolean") {
    errors.push(`${path} must be a boolean`);
    return undefined;
  }
  return value;
}

export function readOptionalNumberAtPath(
  value: unknown,
  path: string,
  errors: string[],
  minimum = 0
): number | undefined {
  if (value === undefined) {
    return undefined;
  }
  if (!isFiniteNumber(value)) {
    errors.push(`${path} must be a number`);
    return undefined;
  }
  if (value < minimum) {
    errors.push(`${path} must be >= ${minimum}`);
    return undefined;
  }
  return value;
}
