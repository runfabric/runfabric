import {
  AwsQueueFunctionResponseTypeEnum,
  TriggerEnum
} from "@runfabric/core";
import type { TriggerConfig } from "@runfabric/core";
import {
  isFiniteNumber,
  isRecord,
  isStringArray
} from "./shared";

function validateQueueTriggerAtPath(trigger: TriggerConfig, path: string, errors: string[]): void {
  if (typeof trigger.queue !== "string" || trigger.queue.trim().length === 0) {
    errors.push(`${path}.queue must be a non-empty string`);
  }

  for (const field of ["batchSize", "maximumBatchingWindowSeconds", "maximumConcurrency"] as const) {
    const value = trigger[field];
    if (value === undefined) {
      continue;
    }
    if (!isFiniteNumber(value)) {
      errors.push(`${path}.${field} must be a number`);
      continue;
    }
    if (value < 0) {
      errors.push(`${path}.${field} must be >= 0`);
    }
  }

  if (trigger.enabled !== undefined && typeof trigger.enabled !== "boolean") {
    errors.push(`${path}.enabled must be a boolean`);
  }

  if (trigger.functionResponseType !== undefined) {
    if (typeof trigger.functionResponseType !== "string") {
      errors.push(`${path}.functionResponseType must be a string`);
    } else if (trigger.functionResponseType !== AwsQueueFunctionResponseTypeEnum.ReportBatchItemFailures) {
      errors.push(`${path}.functionResponseType must be ReportBatchItemFailures`);
    } else {
      trigger.functionResponseType = AwsQueueFunctionResponseTypeEnum.ReportBatchItemFailures;
    }
  }
}

function validateStorageTriggerAtPath(trigger: TriggerConfig, path: string, errors: string[]): void {
  if (typeof trigger.bucket !== "string" || trigger.bucket.trim().length === 0) {
    errors.push(`${path}.bucket must be a non-empty string`);
  }
  if (!isStringArray(trigger.events) || trigger.events.length === 0) {
    errors.push(`${path}.events must be an array with at least one event string`);
  }
  if (trigger.prefix !== undefined && (typeof trigger.prefix !== "string" || trigger.prefix.trim().length === 0)) {
    errors.push(`${path}.prefix must be a non-empty string`);
  }
  if (trigger.suffix !== undefined && (typeof trigger.suffix !== "string" || trigger.suffix.trim().length === 0)) {
    errors.push(`${path}.suffix must be a non-empty string`);
  }
  if (trigger.existingBucket !== undefined && typeof trigger.existingBucket !== "boolean") {
    errors.push(`${path}.existingBucket must be a boolean`);
  }
}

function validateEventBridgeTriggerAtPath(trigger: TriggerConfig, path: string, errors: string[]): void {
  if (trigger.bus !== undefined && (typeof trigger.bus !== "string" || trigger.bus.trim().length === 0)) {
    errors.push(`${path}.bus must be a non-empty string`);
  }
  if (!isRecord(trigger.pattern)) {
    errors.push(`${path}.pattern must be an object`);
  }
}

function validatePubSubTriggerAtPath(trigger: TriggerConfig, path: string, errors: string[]): void {
  if (typeof trigger.topic !== "string" || trigger.topic.trim().length === 0) {
    errors.push(`${path}.topic must be a non-empty string`);
  }
  if (
    trigger.subscription !== undefined &&
    (typeof trigger.subscription !== "string" || trigger.subscription.trim().length === 0)
  ) {
    errors.push(`${path}.subscription must be a non-empty string`);
  }
}

function validateKafkaTriggerAtPath(trigger: TriggerConfig, path: string, errors: string[]): void {
  if (!isStringArray(trigger.brokers) || trigger.brokers.length === 0) {
    errors.push(`${path}.brokers must be an array with at least one broker string`);
  }
  if (typeof trigger.topic !== "string" || trigger.topic.trim().length === 0) {
    errors.push(`${path}.topic must be a non-empty string`);
  }
  if (typeof trigger.groupId !== "string" || trigger.groupId.trim().length === 0) {
    errors.push(`${path}.groupId must be a non-empty string`);
  }
}

function validateRabbitMqTriggerAtPath(trigger: TriggerConfig, path: string, errors: string[]): void {
  if (typeof trigger.queue !== "string" || trigger.queue.trim().length === 0) {
    errors.push(`${path}.queue must be a non-empty string`);
  }
  if (
    trigger.exchange !== undefined &&
    (typeof trigger.exchange !== "string" || trigger.exchange.trim().length === 0)
  ) {
    errors.push(`${path}.exchange must be a non-empty string`);
  }
  if (
    trigger.routingKey !== undefined &&
    (typeof trigger.routingKey !== "string" || trigger.routingKey.trim().length === 0)
  ) {
    errors.push(`${path}.routingKey must be a non-empty string`);
  }
}

function parseTriggerType(value: string): TriggerEnum | null {
  const normalized = value.trim().toLowerCase();
  if (normalized === TriggerEnum.Http) {
    return TriggerEnum.Http;
  }
  if (normalized === TriggerEnum.Cron) {
    return TriggerEnum.Cron;
  }
  if (normalized === TriggerEnum.Queue) {
    return TriggerEnum.Queue;
  }
  if (normalized === TriggerEnum.Storage) {
    return TriggerEnum.Storage;
  }
  if (normalized === TriggerEnum.EventBridge) {
    return TriggerEnum.EventBridge;
  }
  if (normalized === TriggerEnum.PubSub) {
    return TriggerEnum.PubSub;
  }
  if (normalized === TriggerEnum.Kafka) {
    return TriggerEnum.Kafka;
  }
  if (normalized === TriggerEnum.RabbitMq) {
    return TriggerEnum.RabbitMq;
  }
  return null;
}

function parseTriggerField(
  trigger: TriggerConfig,
  path: string,
  field: string,
  rawValue: unknown,
  errors: string[]
): void {
  if (typeof rawValue === "string") {
    trigger[field] = rawValue.trim();
    return;
  }
  if (typeof rawValue === "number" || typeof rawValue === "boolean") {
    trigger[field] = rawValue;
    return;
  }
  if (Array.isArray(rawValue)) {
    if (!isStringArray(rawValue)) {
      errors.push(`${path}.${field} must be an array of non-empty strings`);
      return;
    }
    trigger[field] = rawValue.map((entry) => entry.trim());
    return;
  }
  if (isRecord(rawValue)) {
    trigger[field] = rawValue;
    return;
  }
  errors.push(`${path}.${field} has an unsupported value`);
}

function validateTriggerByType(trigger: TriggerConfig, path: string, errors: string[]): void {
  switch (trigger.type) {
    case TriggerEnum.Queue:
      validateQueueTriggerAtPath(trigger, path, errors);
      return;
    case TriggerEnum.Storage:
      validateStorageTriggerAtPath(trigger, path, errors);
      return;
    case TriggerEnum.EventBridge:
      validateEventBridgeTriggerAtPath(trigger, path, errors);
      return;
    case TriggerEnum.PubSub:
      validatePubSubTriggerAtPath(trigger, path, errors);
      return;
    case TriggerEnum.Kafka:
      validateKafkaTriggerAtPath(trigger, path, errors);
      return;
    case TriggerEnum.RabbitMq:
      validateRabbitMqTriggerAtPath(trigger, path, errors);
      return;
    default:
      return;
  }
}

function parseTriggerAtPath(item: unknown, itemPath: string, errors: string[]): TriggerConfig | null {
  if (!isRecord(item)) {
    errors.push(`${itemPath} must be an object`);
    return null;
  }

  const trigger: TriggerConfig = { type: TriggerEnum.Http };
  for (const [field, rawValue] of Object.entries(item)) {
    if (field === "type") {
      if (typeof rawValue !== "string" || rawValue.trim().length === 0) {
        errors.push(`${itemPath}.type must be a non-empty string`);
        continue;
      }
      const parsedType = parseTriggerType(rawValue);
      if (!parsedType) {
        errors.push(
          `${itemPath}.type must be one of: http, cron, queue, storage, eventbridge, pubsub, kafka, rabbitmq`
        );
        continue;
      }
      trigger.type = parsedType;
      continue;
    }

    parseTriggerField(trigger, itemPath, field, rawValue, errors);
  }

  validateTriggerByType(trigger, itemPath, errors);
  return trigger;
}

export function readTriggerArrayAtPath(
  value: unknown,
  path: string,
  errors: string[],
  minSize = 1
): TriggerConfig[] {
  if (!Array.isArray(value)) {
    errors.push(`${path} must be an array`);
    return [];
  }

  const triggers: TriggerConfig[] = [];
  for (let index = 0; index < value.length; index += 1) {
    const parsed = parseTriggerAtPath(value[index], `${path}[${index}]`, errors);
    if (parsed) {
      triggers.push(parsed);
    }
  }

  if (triggers.length < minSize) {
    errors.push(`${path} must contain at least ${minSize} trigger(s)`);
  }

  return triggers;
}

export function readTriggerArray(source: Record<string, unknown>, errors: string[]): TriggerConfig[] {
  return readTriggerArrayAtPath(source.triggers, "triggers", errors, 1);
}
