import type { ProviderCapabilities } from "@runfabric/core";

export const digitalOceanFunctionsCapabilities: ProviderCapabilities = {
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
  customRuntime: false,
  backgroundJobs: false,
  websockets: false,
  supportedRuntimes: ["nodejs"],
  maxTimeoutSeconds: 900,
  maxMemoryMB: 3072
};
