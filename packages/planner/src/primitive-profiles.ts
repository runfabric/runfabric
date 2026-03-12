import type { ProviderPrimitiveProfile } from "@runfabric/core";

export const primitiveProfiles: Record<string, ProviderPrimitiveProfile> = {
  "aws-lambda": {
    compute: true,
    functionRuntime: true,
    eventTriggers: true,
    workflowOrchestration: true,
    queue: true,
    eventBus: true,
    scheduler: true,
    stateStorage: true,
    observability: true
  },
  "gcp-functions": {
    compute: true,
    functionRuntime: true,
    eventTriggers: true,
    workflowOrchestration: true,
    queue: true,
    eventBus: true,
    scheduler: true,
    stateStorage: true,
    observability: true
  },
  "azure-functions": {
    compute: true,
    functionRuntime: true,
    eventTriggers: true,
    workflowOrchestration: true,
    queue: true,
    eventBus: true,
    scheduler: true,
    stateStorage: true,
    observability: true
  },
  kubernetes: {
    compute: true,
    functionRuntime: true,
    eventTriggers: true,
    workflowOrchestration: true,
    queue: false,
    eventBus: false,
    scheduler: true,
    stateStorage: true,
    observability: true
  },
  "cloudflare-workers": {
    compute: true,
    functionRuntime: true,
    eventTriggers: true,
    workflowOrchestration: false,
    queue: false,
    eventBus: false,
    scheduler: true,
    stateStorage: true,
    observability: true
  },
  vercel: {
    compute: true,
    functionRuntime: true,
    eventTriggers: true,
    workflowOrchestration: false,
    queue: false,
    eventBus: false,
    scheduler: true,
    stateStorage: false,
    observability: true
  },
  netlify: {
    compute: true,
    functionRuntime: true,
    eventTriggers: true,
    workflowOrchestration: false,
    queue: false,
    eventBus: false,
    scheduler: true,
    stateStorage: false,
    observability: true
  },
  "alibaba-fc": {
    compute: true,
    functionRuntime: true,
    eventTriggers: true,
    workflowOrchestration: true,
    queue: true,
    eventBus: true,
    scheduler: true,
    stateStorage: true,
    observability: true
  },
  "digitalocean-functions": {
    compute: true,
    functionRuntime: true,
    eventTriggers: true,
    workflowOrchestration: false,
    queue: false,
    eventBus: false,
    scheduler: true,
    stateStorage: false,
    observability: true
  },
  "fly-machines": {
    compute: true,
    functionRuntime: true,
    eventTriggers: true,
    workflowOrchestration: true,
    queue: false,
    eventBus: false,
    scheduler: false,
    stateStorage: true,
    observability: true
  },
  "ibm-openwhisk": {
    compute: true,
    functionRuntime: true,
    eventTriggers: true,
    workflowOrchestration: false,
    queue: false,
    eventBus: false,
    scheduler: true,
    stateStorage: false,
    observability: true
  }
};
