import type { ProviderCapabilities } from "@runfabric/core";

export const gcpFunctionsCapabilities: ProviderCapabilities = {
  http: true,
  cron: true,
  queue: true,
  storageEvent: true,
  eventbridge: false,
  pubsub: true,
  kafka: false,
  rabbitmq: false,
  streamingResponse: false,
  edgeRuntime: false,
  containerImage: true,
  customRuntime: true,
  backgroundJobs: true,
  websockets: false,
  supportedRuntimes: ["nodejs", "python", "go", "java", "dotnet"],
  engineRuntime: "custom-runtime",
  maxTimeoutSeconds: 3600,
  maxMemoryMB: 32768
};
