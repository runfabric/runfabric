import { PROVIDER_IDS, type ProviderCapabilities } from "@runfabric/core";
import { capabilityMatrix } from "@runfabric/planner";
import type { InitTemplateName } from "./types";

const templateCapabilityMap: Record<InitTemplateName, keyof ProviderCapabilities> = {
  api: "http",
  worker: "http",
  queue: "queue",
  cron: "cron",
  storage: "storageEvent",
  eventbridge: "eventbridge",
  pubsub: "pubsub",
  kafka: "kafka",
  rabbitmq: "rabbitmq"
};

const allTemplates: InitTemplateName[] = [
  "api",
  "worker",
  "queue",
  "cron",
  "storage",
  "eventbridge",
  "pubsub",
  "kafka",
  "rabbitmq"
];

function hasAnyProviderSupport(capability: keyof ProviderCapabilities): boolean {
  return Object.values(capabilityMatrix).some((providerCapabilities) => providerCapabilities[capability] === true);
}

export function isTemplateSupportedByProvider(template: InitTemplateName, provider: string): boolean {
  const capability = templateCapabilityMap[template];
  if (!hasAnyProviderSupport(capability)) {
    return false;
  }
  const capabilities = capabilityMatrix[provider];
  if (!capabilities) {
    return false;
  }
  return capabilities[capability] === true;
}

export function supportedTemplatesForAnyProvider(): InitTemplateName[] {
  return allTemplates.filter((template) =>
    hasAnyProviderSupport(templateCapabilityMap[template])
  );
}

export function supportedTemplatesForProvider(provider: string): InitTemplateName[] {
  return allTemplates.filter((template) => isTemplateSupportedByProvider(template, provider));
}

export function supportedProvidersForTemplate(
  template: InitTemplateName
): Array<(typeof PROVIDER_IDS)[number]> {
  return PROVIDER_IDS.filter((provider) =>
    isTemplateSupportedByProvider(template, provider)
  );
}
