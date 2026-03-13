import type { ProviderCapabilities } from "@runfabric/core";

export const vercelCapabilities: ProviderCapabilities = {
  http: true,
  cron: true,
  queue: false,
  storageEvent: false,
  eventbridge: false,
  pubsub: false,
  kafka: false,
  rabbitmq: false,
  streamingResponse: true,
  edgeRuntime: true,
  containerImage: false,
  customRuntime: false,
  backgroundJobs: false,
  websockets: false,
  supportedRuntimes: ["nodejs"]
};
