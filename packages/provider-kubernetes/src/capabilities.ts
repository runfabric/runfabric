import type { ProviderCapabilities } from "@runfabric/core";

export const kubernetesCapabilities: ProviderCapabilities = {
  http: true,
  cron: true,
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
  supportedRuntimes: ["nodejs", "python", "go", "java", "rust", "dotnet"],
  engineRuntime: "container"
};
