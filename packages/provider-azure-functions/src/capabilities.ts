import type { ProviderCapabilities } from "@runfabric/core";

export const azureFunctionsCapabilities: ProviderCapabilities = {
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
  containerImage: false,
  customRuntime: false,
  backgroundJobs: true,
  websockets: false,
  supportedRuntimes: ["nodejs", "python", "java", "dotnet"],
  engineRuntime: "unsupported",
  maxTimeoutSeconds: 600,
  maxMemoryMB: 4096
};
