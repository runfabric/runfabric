import type { ProviderCapabilities } from "@runfabric/core";

export const flyMachinesCapabilities: ProviderCapabilities = {
  http: true,
  cron: false,
  queue: false,
  storageEvent: false,
  eventbridge: false,
  pubsub: false,
  kafka: false,
  rabbitmq: false,
  streamingResponse: true,
  edgeRuntime: false,
  containerImage: true,
  customRuntime: true,
  backgroundJobs: true,
  websockets: true,
  maxTimeoutSeconds: 3600,
  maxMemoryMB: 8192
};
