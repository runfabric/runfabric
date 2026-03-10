export interface ProviderCapabilities {
  http: boolean;
  cron: boolean;
  queue: boolean;
  storageEvent: boolean;
  streamingResponse: boolean;
  edgeRuntime: boolean;
  containerImage: boolean;
  customRuntime: boolean;
  backgroundJobs: boolean;
  websockets: boolean;
  maxTimeoutSeconds?: number;
  maxMemoryMB?: number;
}
