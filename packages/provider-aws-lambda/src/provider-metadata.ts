import {
  AwsIamEffectEnum,
  AwsQueueFunctionResponseTypeEnum,
  TriggerEnum
} from "@runfabric/core";
import type { ProjectConfig } from "@runfabric/core";

interface AwsIamRoleStatement {
  sid?: string;
  effect: AwsIamEffectEnum;
  actions: string[];
  resources: string[];
  condition?: Record<string, unknown>;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function readString(value: unknown): string | undefined {
  return typeof value === "string" && value.trim().length > 0 ? value : undefined;
}

function readNumber(value: unknown): number | undefined {
  return typeof value === "number" && Number.isFinite(value) ? value : undefined;
}

function readBoolean(value: unknown): boolean | undefined {
  return typeof value === "boolean" ? value : undefined;
}

function readQueueFunctionResponseType(value: unknown): AwsQueueFunctionResponseTypeEnum | undefined {
  if (value === AwsQueueFunctionResponseTypeEnum.ReportBatchItemFailures) {
    return AwsQueueFunctionResponseTypeEnum.ReportBatchItemFailures;
  }
  return undefined;
}

export function endpointFromAwsResponse(response: unknown): string | undefined {
  if (!isRecord(response)) {
    return undefined;
  }

  for (const candidate of [response.endpoint, response.url, response.functionUrl, response.FunctionUrl]) {
    const endpoint = readString(candidate);
    if (endpoint) {
      return endpoint;
    }
  }

  if (isRecord(response.FunctionUrlConfig)) {
    return readString(response.FunctionUrlConfig.FunctionUrl);
  }
  return undefined;
}

export function awsResourceMetadata(response: unknown): Record<string, unknown> | undefined {
  if (!isRecord(response)) {
    return undefined;
  }

  const metadata: Record<string, unknown> = {};
  for (const key of ["FunctionArn", "RevisionId", "Version", "Runtime"]) {
    const value = readString(response[key]);
    if (value) {
      metadata[key] = value;
    }
  }
  return Object.keys(metadata).length > 0 ? metadata : undefined;
}

function collectQueueEventSources(project: ProjectConfig): Array<Record<string, unknown>> {
  return project.triggers
    .filter((trigger) => trigger.type === TriggerEnum.Queue)
    .map((trigger) => ({
      queue: readString(trigger.queue),
      batchSize: readNumber(trigger.batchSize),
      maximumBatchingWindowSeconds: readNumber(trigger.maximumBatchingWindowSeconds),
      maximumConcurrency: readNumber(trigger.maximumConcurrency),
      enabled: readBoolean(trigger.enabled) ?? true,
      functionResponseType: readQueueFunctionResponseType(trigger.functionResponseType)
    }))
    .filter((source) => typeof source.queue === "string");
}

function collectStorageEvents(project: ProjectConfig): Array<Record<string, unknown>> {
  return project.triggers
    .filter((trigger) => trigger.type === TriggerEnum.Storage)
    .map((trigger) => {
      const events =
        Array.isArray(trigger.events) && trigger.events.every((entry) => typeof entry === "string")
          ? trigger.events
          : [];
      return {
        bucket: readString(trigger.bucket),
        events,
        prefix: readString(trigger.prefix),
        suffix: readString(trigger.suffix),
        existingBucket: readBoolean(trigger.existingBucket) ?? true
      };
    })
    .filter((source) => typeof source.bucket === "string" && source.events.length > 0);
}

function collectEventBridgeRules(project: ProjectConfig): Array<Record<string, unknown>> {
  return project.triggers
    .filter((trigger) => trigger.type === TriggerEnum.EventBridge)
    .map((trigger) => ({
      bus: readString(trigger.bus) || "default",
      pattern: isRecord(trigger.pattern) ? trigger.pattern : undefined
    }))
    .filter((rule) => isRecord(rule.pattern));
}

function collectIamRoleStatements(project: ProjectConfig): AwsIamRoleStatement[] {
  const extension = project.extensions?.["aws-lambda"];
  if (!extension || typeof extension !== "object") {
    return [];
  }

  const iam = (extension as { iam?: unknown }).iam;
  if (!iam || !isRecord(iam)) {
    return [];
  }

  const role = iam.role;
  if (!role || !isRecord(role) || !Array.isArray(role.statements)) {
    return [];
  }

  return role.statements.filter((statement): statement is AwsIamRoleStatement => {
    if (!isRecord(statement)) {
      return false;
    }
    const effect = readString(statement.effect);
    if (!effect || ![AwsIamEffectEnum.Allow, AwsIamEffectEnum.Deny].includes(effect as AwsIamEffectEnum)) {
      return false;
    }

    const actions = statement.actions;
    const resources = statement.resources;
    const actionsValid = Array.isArray(actions) && actions.every((entry) => typeof entry === "string");
    const resourcesValid = Array.isArray(resources) && resources.every((entry) => typeof entry === "string");
    return actionsValid && resourcesValid;
  });
}

function collectFunctionEnv(project: ProjectConfig): Record<string, string> {
  return { ...(project.env || {}) };
}

function sanitizeIdentifier(value: string): string {
  return value.replace(/[^a-zA-Z0-9-_./]/g, "-");
}

function resourceName(entry: unknown): string | undefined {
  return isRecord(entry) ? readString(entry.name) : undefined;
}

export function collectResourceAddresses(project: ProjectConfig, region: string): Record<string, string> {
  const accountId = "000000000000";
  const out: Record<string, string> = {};
  const resources = project.resources;
  if (!resources) {
    return out;
  }

  for (const queue of resources.queues || []) {
    const name = resourceName(queue);
    if (name) {
      out[`queue.${name}`] = `arn:aws:sqs:${region}:${accountId}:${sanitizeIdentifier(name)}`;
    }
  }
  for (const bucket of resources.buckets || []) {
    const name = resourceName(bucket);
    if (name) {
      out[`bucket.${name}`] = `arn:aws:s3:::${sanitizeIdentifier(name)}`;
    }
  }
  for (const topic of resources.topics || []) {
    const name = resourceName(topic);
    if (name) {
      out[`topic.${name}`] = `arn:aws:sns:${region}:${accountId}:${sanitizeIdentifier(name)}`;
    }
  }
  for (const database of resources.databases || []) {
    const name = resourceName(database);
    if (name) {
      out[`database.${name}`] = `arn:aws:rds:${region}:${accountId}:db:${sanitizeIdentifier(name)}`;
    }
  }
  return out;
}

export function collectWorkflowAddresses(
  project: ProjectConfig,
  region: string,
  stage: string
): Record<string, string> {
  const accountId = "000000000000";
  const out: Record<string, string> = {};

  for (const workflow of project.workflows || []) {
    if (!workflow.name || workflow.name.trim().length === 0) {
      continue;
    }
    const name = sanitizeIdentifier(workflow.name.trim());
    out[`workflow.${workflow.name}`] =
      `arn:aws:states:${region}:${accountId}:stateMachine:${sanitizeIdentifier(project.service)}-${sanitizeIdentifier(stage)}-${name}`;
  }

  return out;
}

export function collectSecretReferences(project: ProjectConfig, region: string): Record<string, string> {
  const accountId = "000000000000";
  const out: Record<string, string> = {};

  for (const [key, value] of Object.entries(project.secrets || {})) {
    if (typeof value !== "string" || !value.startsWith("secret://")) {
      continue;
    }
    const ref = value.slice("secret://".length).trim();
    if (!ref) {
      continue;
    }
    out[key] = `arn:aws:secretsmanager:${region}:${accountId}:secret:${sanitizeIdentifier(ref)}`;
  }

  return out;
}

export function createAwsDeployMetadata(project: ProjectConfig, region: string, stage: string) {
  return {
    queueEventSources: collectQueueEventSources(project),
    storageEvents: collectStorageEvents(project),
    eventBridgeRules: collectEventBridgeRules(project),
    iamRoleStatements: collectIamRoleStatements(project),
    functionEnv: collectFunctionEnv(project),
    resourceAddresses: collectResourceAddresses(project, region),
    workflowAddresses: collectWorkflowAddresses(project, region, stage),
    secretReferences: collectSecretReferences(project, region)
  };
}
