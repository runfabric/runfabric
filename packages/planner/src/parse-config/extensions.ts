import { AwsIamEffectEnum } from "@runfabric/core";
import type { ProjectConfig } from "@runfabric/core";
import { isRecord, isScalar, isStringArray } from "./shared";

type ExtensionValueType = "string" | "number" | "boolean" | "object";

const providerExtensionSchema: Record<string, Record<string, ExtensionValueType>> = {
  "aws-lambda": {
    stage: "string",
    region: "string",
    roleArn: "string",
    functionName: "string",
    runtime: "string",
    iam: "object"
  },
  "gcp-functions": {
    region: "string"
  },
  "azure-functions": {
    functionApp: "string",
    functionAppName: "string",
    routePrefix: "string"
  },
  kubernetes: {
    namespace: "string",
    context: "string",
    deploymentName: "string",
    serviceName: "string",
    ingressHost: "string"
  },
  "cloudflare-workers": {
    scriptName: "string"
  },
  vercel: {
    projectName: "string"
  },
  netlify: {
    siteName: "string"
  },
  "alibaba-fc": {
    region: "string"
  },
  "digitalocean-functions": {
    namespace: "string",
    region: "string"
  },
  "fly-machines": {
    appName: "string",
    region: "string"
  },
  "ibm-openwhisk": {
    namespace: "string"
  }
};

function validateAwsIamStatementAtPath(statementValue: unknown, path: string, errors: string[]): void {
  if (!isRecord(statementValue)) {
    errors.push(`${path} must be an object`);
    return;
  }

  const statement = statementValue;
  for (const key of Object.keys(statement)) {
    if (!["sid", "effect", "actions", "resources", "condition"].includes(key)) {
      errors.push(`${path}.${key} is not a supported field`);
    }
  }

  if ("sid" in statement && (typeof statement.sid !== "string" || statement.sid.trim().length === 0)) {
    errors.push(`${path}.sid must be a non-empty string`);
  }

  if (
    typeof statement.effect !== "string" ||
    ![AwsIamEffectEnum.Allow, AwsIamEffectEnum.Deny].includes(statement.effect as AwsIamEffectEnum)
  ) {
    errors.push(`${path}.effect must be Allow or Deny`);
  }

  if (!isStringArray(statement.actions) || statement.actions.length === 0) {
    errors.push(`${path}.actions must be an array with at least one action`);
  }
  if (!isStringArray(statement.resources) || statement.resources.length === 0) {
    errors.push(`${path}.resources must be an array with at least one resource`);
  }
  if ("condition" in statement && !isRecord(statement.condition)) {
    errors.push(`${path}.condition must be an object`);
  }
}

function validateAwsIamConfigAtPath(value: unknown, path: string, errors: string[]): void {
  if (!isRecord(value)) {
    errors.push(`${path} must be an object`);
    return;
  }

  for (const key of Object.keys(value)) {
    if (key !== "role") {
      errors.push(`${path}.${key} is not a supported field`);
    }
  }

  if (!("role" in value) || value.role === undefined) {
    return;
  }
  if (!isRecord(value.role)) {
    errors.push(`${path}.role must be an object`);
    return;
  }

  for (const key of Object.keys(value.role)) {
    if (key !== "statements") {
      errors.push(`${path}.role.${key} is not a supported field`);
    }
  }

  if (!("statements" in value.role) || value.role.statements === undefined) {
    return;
  }
  if (!Array.isArray(value.role.statements)) {
    errors.push(`${path}.role.statements must be an array`);
    return;
  }

  value.role.statements.forEach((statement, index) => {
    validateAwsIamStatementAtPath(statement, `${path}.role.statements[${index}]`, errors);
  });
}

function validateProviderExtension(
  provider: string,
  values: unknown,
  providers: string[],
  errors: string[]
): void {
  if (!providers.includes(provider)) {
    errors.push(`extensions.${provider} is set but provider is not listed in providers`);
    return;
  }

  const schema = providerExtensionSchema[provider];
  if (!schema) {
    return;
  }

  if (!values || !isRecord(values)) {
    errors.push(`extensions.${provider} must be an object`);
    return;
  }

  for (const [field, value] of Object.entries(values)) {
    const expectedType = schema[field];
    if (!expectedType) {
      errors.push(`extensions.${provider}.${field} is not a supported field`);
      continue;
    }

    if (expectedType === "object") {
      if (!isRecord(value)) {
        errors.push(`extensions.${provider}.${field} must be an object`);
        continue;
      }
      if (provider === "aws-lambda" && field === "iam") {
        validateAwsIamConfigAtPath(value, `extensions.${provider}.${field}`, errors);
      }
      continue;
    }

    if (typeof value !== expectedType) {
      errors.push(`extensions.${provider}.${field} must be ${expectedType}, received ${typeof value}`);
    }
  }
}

export function validateExtensionTypes(
  extensions: ProjectConfig["extensions"],
  providers: string[],
  errors: string[]
): void {
  if (!extensions) {
    return;
  }
  for (const [provider, values] of Object.entries(extensions)) {
    validateProviderExtension(provider, values, providers, errors);
  }
}

function cloneExtensionValueAtPath(value: unknown, path: string, errors: string[]): unknown {
  if (isScalar(value)) {
    return value;
  }
  if (Array.isArray(value)) {
    return value.map((entry, index) => cloneExtensionValueAtPath(entry, `${path}[${index}]`, errors));
  }
  if (isRecord(value)) {
    const cloned: Record<string, unknown> = {};
    for (const [entryKey, entryValue] of Object.entries(value)) {
      cloned[entryKey] = cloneExtensionValueAtPath(entryValue, `${path}.${entryKey}`, errors);
    }
    return cloned;
  }

  errors.push(`${path} has an unsupported value`);
  return undefined;
}

export function readExtensionsAtPath(
  value: unknown,
  path: string,
  errors: string[]
): ProjectConfig["extensions"] {
  if (value === undefined) {
    return undefined;
  }
  if (!isRecord(value)) {
    errors.push(`${path} must be an object`);
    return undefined;
  }

  const out: NonNullable<ProjectConfig["extensions"]> = {};
  for (const [provider, providerConfig] of Object.entries(value)) {
    if (!isRecord(providerConfig)) {
      errors.push(`${path}.${provider} must be an object`);
      continue;
    }

    const extensionValues: Record<string, unknown> = {};
    for (const [field, fieldValue] of Object.entries(providerConfig)) {
      const clonedValue = cloneExtensionValueAtPath(fieldValue, `${path}.${provider}.${field}`, errors);
      if (clonedValue !== undefined) {
        extensionValues[field] = clonedValue;
      }
    }
    out[provider] = extensionValues;
  }

  return out;
}

export function readExtensions(
  source: Record<string, unknown>,
  errors: string[]
): ProjectConfig["extensions"] {
  return readExtensionsAtPath(source.extensions, "extensions", errors);
}

export function deepMergeRecords(
  base: Record<string, unknown> | undefined,
  override: Record<string, unknown> | undefined
): Record<string, unknown> {
  const merged: Record<string, unknown> = {
    ...(base || {})
  };
  if (!override) {
    return merged;
  }

  for (const [key, value] of Object.entries(override)) {
    const baseValue = merged[key];
    merged[key] = isRecord(baseValue) && isRecord(value) ? deepMergeRecords(baseValue, value) : value;
  }

  return merged;
}

export function mergeExtensions(
  base: ProjectConfig["extensions"],
  override: ProjectConfig["extensions"]
): ProjectConfig["extensions"] {
  if (!base && !override) {
    return undefined;
  }

  const merged: NonNullable<ProjectConfig["extensions"]> = { ...(base || {}) };
  if (!override) {
    return merged;
  }

  for (const [provider, values] of Object.entries(override)) {
    if (!values || !isRecord(values)) {
      continue;
    }
    const current = merged[provider];
    merged[provider] = deepMergeRecords(isRecord(current) ? current : undefined, values);
  }

  return merged;
}

export function mergeStateConfig(
  base: ProjectConfig["state"],
  override: ProjectConfig["state"]
): ProjectConfig["state"] {
  if (!base && !override) {
    return undefined;
  }

  const merged = deepMergeRecords(
    (base || {}) as Record<string, unknown>,
    (override || {}) as Record<string, unknown>
  );
  return merged as NonNullable<ProjectConfig["state"]>;
}
