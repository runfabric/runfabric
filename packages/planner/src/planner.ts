import type {
  PrimitiveCompatibilityReport,
  ProjectConfig,
  ProviderCapabilities,
  TriggerConfig
} from "@runfabric/core";
import { TriggerEnum } from "@runfabric/core";
import { capabilityMatrix } from "./capability-matrix";
import { createPortabilityDiagnostics, type PortabilityDiagnostics } from "./portability";
import { createPrimitiveCompatibilityReport } from "./primitive-compatibility";
import { primitiveProfiles } from "./primitive-profiles";

export interface ProviderPlan {
  provider: string;
  capabilities?: ProviderCapabilities;
  warnings: string[];
  errors: string[];
}

export interface PlanningResult {
  ok: boolean;
  project: ProjectConfig;
  providerPlans: ProviderPlan[];
  portability: PortabilityDiagnostics;
  primitiveCompatibility: PrimitiveCompatibilityReport;
  warnings: string[];
  errors: string[];
}

function isNonEmptyString(value: unknown): value is string {
  return typeof value === "string" && value.trim().length > 0;
}

function hasStringArrayValues(value: unknown): value is string[] {
  return Array.isArray(value) && value.every((entry) => typeof entry === "string" && entry.trim().length > 0);
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function validateTriggerShape(trigger: TriggerConfig): string | null {
  if (trigger.type === TriggerEnum.Queue) {
    if (!isNonEmptyString(trigger.queue)) {
      return "queue trigger requires queue";
    }
  }

  if (trigger.type === TriggerEnum.Storage) {
    if (!isNonEmptyString(trigger.bucket)) {
      return "storage trigger requires bucket";
    }
    if (!hasStringArrayValues(trigger.events) || trigger.events.length === 0) {
      return "storage trigger requires events";
    }
  }

  if (trigger.type === TriggerEnum.EventBridge) {
    if (!isRecord(trigger.pattern)) {
      return "eventbridge trigger requires pattern object";
    }
  }

  if (trigger.type === TriggerEnum.PubSub) {
    if (!isNonEmptyString(trigger.topic)) {
      return "pubsub trigger requires topic";
    }
  }

  if (trigger.type === TriggerEnum.Kafka) {
    if (!hasStringArrayValues(trigger.brokers) || trigger.brokers.length === 0) {
      return "kafka trigger requires brokers";
    }
    if (!isNonEmptyString(trigger.topic)) {
      return "kafka trigger requires topic";
    }
    if (!isNonEmptyString(trigger.groupId)) {
      return "kafka trigger requires groupId";
    }
  }

  if (trigger.type === TriggerEnum.RabbitMq) {
    if (!isNonEmptyString(trigger.queue)) {
      return "rabbitmq trigger requires queue";
    }
  }

  return null;
}

function validateTriggerSupport(trigger: TriggerConfig, capabilities: ProviderCapabilities): string | null {
  if (trigger.type === TriggerEnum.Http && !capabilities.http) {
    return "http trigger is not supported";
  }
  if (trigger.type === TriggerEnum.Cron && !capabilities.cron) {
    return "cron trigger is not supported";
  }
  if (trigger.type === TriggerEnum.Queue && !capabilities.queue) {
    return "queue trigger is not supported";
  }
  if (trigger.type === TriggerEnum.Storage && !capabilities.storageEvent) {
    return "storage trigger is not supported";
  }
  if (trigger.type === TriggerEnum.EventBridge && !capabilities.eventbridge) {
    return "eventbridge trigger is not supported";
  }
  if (trigger.type === TriggerEnum.PubSub && !capabilities.pubsub) {
    return "pubsub trigger is not supported";
  }
  if (trigger.type === TriggerEnum.Kafka && !capabilities.kafka) {
    return "kafka trigger is not supported";
  }
  if (trigger.type === TriggerEnum.RabbitMq && !capabilities.rabbitmq) {
    return "rabbitmq trigger is not supported";
  }
  return null;
}

function pushProviderError(
  providerPlan: ProviderPlan,
  provider: string,
  detail: string,
  errors: string[]
): void {
  const message = `${provider}: ${detail}`;
  providerPlan.errors.push(message);
  errors.push(message);
}

function evaluateProviderTriggers(
  project: ProjectConfig,
  provider: string,
  capabilities: ProviderCapabilities,
  providerPlan: ProviderPlan,
  errors: string[]
): void {
  for (const trigger of project.triggers) {
    const triggerShapeError = validateTriggerShape(trigger);
    if (triggerShapeError) {
      pushProviderError(providerPlan, provider, triggerShapeError, errors);
    }

    const triggerError = validateTriggerSupport(trigger, capabilities);
    if (triggerError) {
      pushProviderError(providerPlan, provider, triggerError, errors);
    }
  }
}

function evaluateProviderResources(
  project: ProjectConfig,
  provider: string,
  capabilities: ProviderCapabilities,
  providerPlan: ProviderPlan,
  errors: string[]
): void {
  if (project.resources?.timeout && capabilities.maxTimeoutSeconds && project.resources.timeout > capabilities.maxTimeoutSeconds) {
    pushProviderError(
      providerPlan,
      provider,
      `timeout ${project.resources.timeout}s exceeds max ${capabilities.maxTimeoutSeconds}s`,
      errors
    );
  }

  if (project.resources?.memory && capabilities.maxMemoryMB && project.resources.memory > capabilities.maxMemoryMB) {
    pushProviderError(
      providerPlan,
      provider,
      `memory ${project.resources.memory}MB exceeds max ${capabilities.maxMemoryMB}MB`,
      errors
    );
  }
}

function evaluateProviderRuntimeSupport(
  project: ProjectConfig,
  provider: string,
  capabilities: ProviderCapabilities,
  providerPlan: ProviderPlan,
  errors: string[]
): void {
  const supported = capabilities.supportedRuntimes;
  const supportedLabel = supported.join(", ");
  if (!supported.includes(project.runtime)) {
    pushProviderError(
      providerPlan,
      provider,
      `runtime ${project.runtime} is not supported (supported: ${supportedLabel})`,
      errors
    );
  }

  for (const fn of project.functions || []) {
    if (!fn.runtime || supported.includes(fn.runtime)) {
      continue;
    }
    pushProviderError(
      providerPlan,
      provider,
      `function ${fn.name} runtime ${fn.runtime} is not supported (supported: ${supportedLabel})`,
      errors
    );
  }
}

function evaluateProviderEngineModeSupport(
  project: ProjectConfig,
  provider: string,
  capabilities: ProviderCapabilities,
  providerPlan: ProviderPlan,
  errors: string[]
): void {
  if (project.runtimeMode !== "engine") {
    return;
  }
  if (capabilities.engineRuntime !== "unsupported") {
    return;
  }
  pushProviderError(
    providerPlan,
    provider,
    "runtimeMode engine is not supported for this provider (engineRuntime=unsupported)",
    errors
  );
}

function createProviderPlanEntry(
  project: ProjectConfig,
  provider: string,
  errors: string[]
): ProviderPlan {
  const capabilities = capabilityMatrix[provider];
  const providerPlan: ProviderPlan = {
    provider,
    capabilities,
    warnings: [],
    errors: []
  };

  if (!capabilities) {
    const unsupported = `unsupported provider: ${provider}`;
    providerPlan.errors.push(unsupported);
    errors.push(unsupported);
    return providerPlan;
  }

  evaluateProviderTriggers(project, provider, capabilities, providerPlan, errors);
  evaluateProviderResources(project, provider, capabilities, providerPlan, errors);
  evaluateProviderRuntimeSupport(project, provider, capabilities, providerPlan, errors);
  evaluateProviderEngineModeSupport(project, provider, capabilities, providerPlan, errors);
  return providerPlan;
}

function appendPortabilityWarnings(
  portability: PortabilityDiagnostics,
  primitiveCompatibility: PrimitiveCompatibilityReport,
  warnings: string[]
): void {
  if (portability.partiallySupportedTriggerTypes.length > 0) {
    warnings.push(
      `partial portability for triggers: ${portability.partiallySupportedTriggerTypes.join(", ")}`
    );
  }
  if (primitiveCompatibility.partiallySupported.length > 0) {
    warnings.push(
      `partial primitive support: ${primitiveCompatibility.partiallySupported.join(", ")}`
    );
  }
}

export function createPlan(project: ProjectConfig): PlanningResult {
  const providerPlans: ProviderPlan[] = [];
  const warnings: string[] = [];
  const errors: string[] = [];

  for (const provider of project.providers) {
    providerPlans.push(createProviderPlanEntry(project, provider, errors));
  }

  const portability = createPortabilityDiagnostics(
    project,
    Object.fromEntries(providerPlans.map((providerPlan) => [providerPlan.provider, providerPlan.capabilities]))
  );
  const primitiveCompatibility = createPrimitiveCompatibilityReport(
    project.providers,
    Object.fromEntries(project.providers.map((provider) => [provider, primitiveProfiles[provider]]))
  );

  appendPortabilityWarnings(portability, primitiveCompatibility, warnings);

  return {
    ok: errors.length === 0,
    project,
    providerPlans,
    portability,
    primitiveCompatibility,
    warnings,
    errors
  };
}
