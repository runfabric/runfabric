import type { ProviderCapabilities } from "@runfabric/core";

export const awsLambdaCapabilities: ProviderCapabilities = {
  http: true,
  cron: true,
  queue: true,
  storageEvent: true,
  eventbridge: true,
  pubsub: false,
  kafka: false,
  rabbitmq: false,
  streamingResponse: true,
  edgeRuntime: false,
  containerImage: true,
  customRuntime: true,
  backgroundJobs: true,
  websockets: true,
  maxTimeoutSeconds: 900,
  maxMemoryMB: 10240
};
