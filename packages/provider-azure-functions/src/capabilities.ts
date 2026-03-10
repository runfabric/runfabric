import type { ProviderCapabilities } from "@runfabric/core";

export const azureFunctionsCapabilities: ProviderCapabilities = {
  http: true,
  cron: true,
  queue: true,
  storageEvent: true,
  streamingResponse: false,
  edgeRuntime: false,
  containerImage: false,
  customRuntime: false,
  backgroundJobs: true,
  websockets: false,
  maxTimeoutSeconds: 600,
  maxMemoryMB: 4096
};
