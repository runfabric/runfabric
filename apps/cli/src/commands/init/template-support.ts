import type { ProviderCapabilities } from "@runfabric/core";
import { capabilityMatrix } from "@runfabric/planner";
import type { InitTemplateName } from "./types";

const templateCapabilityMap: Record<InitTemplateName, keyof ProviderCapabilities> = {
  api: "http",
  worker: "http",
  queue: "queue",
  cron: "cron"
};

const allTemplates: InitTemplateName[] = ["api", "worker", "queue", "cron"];

function templateCapability(template: InitTemplateName): keyof ProviderCapabilities {
  return templateCapabilityMap[template];
}

export function isTemplateSupportedByProvider(template: InitTemplateName, provider: string): boolean {
  const capabilities = capabilityMatrix[provider];
  if (!capabilities) {
    return true;
  }
  return capabilities[templateCapability(template)] === true;
}

export function supportedTemplatesForProvider(provider: string): InitTemplateName[] {
  return allTemplates.filter((template) => isTemplateSupportedByProvider(template, provider));
}
