import type { ProviderCapabilities } from "@runfabric/core";

export const ibmOpenWhiskCapabilities: ProviderCapabilities = {
  http: true,
  cron: true,
  queue: false,
  storageEvent: false,
  eventbridge: false,
  pubsub: false,
  kafka: false,
  rabbitmq: false,
  streamingResponse: false,
  edgeRuntime: false,
  containerImage: false,
  customRuntime: true,
  backgroundJobs: false,
  websockets: false,
  maxTimeoutSeconds: 600,
  maxMemoryMB: 512
};
