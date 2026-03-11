import type { ProviderCapabilities } from "@runfabric/core";

export const alibabaFcCapabilities: ProviderCapabilities = {
  http: true,
  cron: true,
  queue: true,
  storageEvent: true,
  eventbridge: false,
  pubsub: false,
  kafka: false,
  rabbitmq: false,
  streamingResponse: false,
  edgeRuntime: false,
  containerImage: true,
  customRuntime: true,
  backgroundJobs: true,
  websockets: false,
  maxTimeoutSeconds: 3600,
  maxMemoryMB: 32768
};
