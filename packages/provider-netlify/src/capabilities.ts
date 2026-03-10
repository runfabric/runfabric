import type { ProviderCapabilities } from "@runfabric/core";

export const netlifyCapabilities: ProviderCapabilities = {
  http: true,
  cron: true,
  queue: false,
  storageEvent: false,
  streamingResponse: false,
  edgeRuntime: false,
  containerImage: false,
  customRuntime: false,
  backgroundJobs: false,
  websockets: false
};
