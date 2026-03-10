import type { ProviderCapabilities } from "@runfabric/core";

export const digitalOceanFunctionsCapabilities: ProviderCapabilities = {
  http: true,
  cron: true,
  queue: false,
  storageEvent: false,
  streamingResponse: false,
  edgeRuntime: false,
  containerImage: false,
  customRuntime: false,
  backgroundJobs: false,
  websockets: false,
  maxTimeoutSeconds: 900,
  maxMemoryMB: 3072
};
