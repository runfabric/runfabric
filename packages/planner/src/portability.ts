import type { ProjectConfig, ProviderCapabilities } from "@runfabric/core";

export interface PortabilityDiagnostics {
  requiredTriggerTypes: string[];
  universallySupportedTriggerTypes: string[];
  partiallySupportedTriggerTypes: string[];
  providerGaps: Record<string, string[]>;
}

function supportsTrigger(type: string, capabilities: ProviderCapabilities): boolean {
  if (type === "http") {
    return capabilities.http;
  }
  if (type === "cron") {
    return capabilities.cron;
  }
  if (type === "queue") {
    return capabilities.queue;
  }
  return false;
}

export function createPortabilityDiagnostics(
  project: ProjectConfig,
  providerCapabilities: Record<string, ProviderCapabilities | undefined>
): PortabilityDiagnostics {
  const requiredTriggerTypes = Array.from(new Set(project.triggers.map((trigger) => trigger.type)));
  const providerGaps: Record<string, string[]> = {};

  for (const [provider, capabilities] of Object.entries(providerCapabilities)) {
    if (!capabilities) {
      providerGaps[provider] = ["provider capabilities missing"];
      continue;
    }
    const gaps = requiredTriggerTypes.filter((type) => !supportsTrigger(type, capabilities));
    providerGaps[provider] = gaps;
  }

  const universallySupportedTriggerTypes = requiredTriggerTypes.filter((type) =>
    Object.values(providerCapabilities).every((capabilities) => capabilities && supportsTrigger(type, capabilities))
  );

  const partiallySupportedTriggerTypes = requiredTriggerTypes.filter((type) => {
    const supportStates = Object.values(providerCapabilities).map((capabilities) =>
      capabilities ? supportsTrigger(type, capabilities) : false
    );
    return supportStates.some(Boolean) && supportStates.some((value) => !value);
  });

  return {
    requiredTriggerTypes,
    universallySupportedTriggerTypes,
    partiallySupportedTriggerTypes,
    providerGaps
  };
}
