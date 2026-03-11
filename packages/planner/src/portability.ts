import { TriggerEnum } from "@runfabric/core";
import type { ProjectConfig, ProviderCapabilities } from "@runfabric/core";

export interface PortabilityDiagnostics {
  requiredTriggerTypes: string[];
  universallySupportedTriggerTypes: string[];
  partiallySupportedTriggerTypes: string[];
  providerGaps: Record<string, string[]>;
}

function supportsTrigger(type: TriggerEnum, capabilities: ProviderCapabilities): boolean {
  if (type === TriggerEnum.Http) {
    return capabilities.http;
  }
  if (type === TriggerEnum.Cron) {
    return capabilities.cron;
  }
  if (type === TriggerEnum.Queue) {
    return capabilities.queue;
  }
  if (type === TriggerEnum.Storage) {
    return capabilities.storageEvent;
  }
  if (type === TriggerEnum.EventBridge) {
    return capabilities.eventbridge;
  }
  if (type === TriggerEnum.PubSub) {
    return capabilities.pubsub;
  }
  if (type === TriggerEnum.Kafka) {
    return capabilities.kafka;
  }
  if (type === TriggerEnum.RabbitMq) {
    return capabilities.rabbitmq;
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
