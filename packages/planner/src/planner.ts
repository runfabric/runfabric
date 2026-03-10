import type {
  PrimitiveCompatibilityReport,
  ProjectConfig,
  ProviderCapabilities,
  TriggerConfig
} from "@runfabric/core";
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

function validateTriggerSupport(trigger: TriggerConfig, capabilities: ProviderCapabilities): string | null {
  if (trigger.type === "http" && !capabilities.http) {
    return "http trigger is not supported";
  }
  if (trigger.type === "cron" && !capabilities.cron) {
    return "cron trigger is not supported";
  }
  if (trigger.type === "queue" && !capabilities.queue) {
    return "queue trigger is not supported";
  }
  return null;
}

export function createPlan(project: ProjectConfig): PlanningResult {
  const providerPlans: ProviderPlan[] = [];
  const warnings: string[] = [];
  const errors: string[] = [];

  for (const provider of project.providers) {
    const capabilities = capabilityMatrix[provider];
    const providerPlan: ProviderPlan = {
      provider,
      capabilities,
      warnings: [],
      errors: []
    };

    if (!capabilities) {
      providerPlan.errors.push(`unsupported provider: ${provider}`);
      errors.push(`unsupported provider: ${provider}`);
      providerPlans.push(providerPlan);
      continue;
    }

    for (const trigger of project.triggers) {
      const triggerError = validateTriggerSupport(trigger, capabilities);
      if (triggerError) {
        const message = `${provider}: ${triggerError}`;
        providerPlan.errors.push(message);
        errors.push(message);
      }
    }

    if (project.resources?.timeout && capabilities.maxTimeoutSeconds && project.resources.timeout > capabilities.maxTimeoutSeconds) {
      const message = `${provider}: timeout ${project.resources.timeout}s exceeds max ${capabilities.maxTimeoutSeconds}s`;
      providerPlan.errors.push(message);
      errors.push(message);
    }

    if (project.resources?.memory && capabilities.maxMemoryMB && project.resources.memory > capabilities.maxMemoryMB) {
      const message = `${provider}: memory ${project.resources.memory}MB exceeds max ${capabilities.maxMemoryMB}MB`;
      providerPlan.errors.push(message);
      errors.push(message);
    }

    if (provider === "cloudflare-workers" && project.runtime !== "nodejs") {
      const message = `${provider}: non-node runtimes are not yet supported`;
      providerPlan.warnings.push(message);
      warnings.push(message);
    }

    providerPlans.push(providerPlan);
  }

  const portability = createPortabilityDiagnostics(
    project,
    Object.fromEntries(providerPlans.map((providerPlan) => [providerPlan.provider, providerPlan.capabilities]))
  );
  const primitiveCompatibility = createPrimitiveCompatibilityReport(
    project.providers,
    Object.fromEntries(project.providers.map((provider) => [provider, primitiveProfiles[provider]]))
  );

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
