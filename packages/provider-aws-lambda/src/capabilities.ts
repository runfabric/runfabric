import type { ProviderCapabilities } from "@runfabric/core";

export const awsLambdaCapabilities: ProviderCapabilities = {
  http: true,
  cron: true,
  queue: true,
  storageEvent: true,
  streamingResponse: true,
  edgeRuntime: false,
  containerImage: true,
  customRuntime: true,
  backgroundJobs: true,
  websockets: true,
  maxTimeoutSeconds: 900,
  maxMemoryMB: 10240
};
