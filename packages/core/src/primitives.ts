export const PLATFORM_PRIMITIVES = [
  "compute",
  "functionRuntime",
  "eventTriggers",
  "workflowOrchestration",
  "queue",
  "eventBus",
  "scheduler",
  "stateStorage",
  "observability"
] as const;

export type PlatformPrimitive = (typeof PLATFORM_PRIMITIVES)[number];

export type ProviderPrimitiveProfile = Record<PlatformPrimitive, boolean>;

export interface PrimitiveCompatibilityReport {
  selectedProviders: string[];
  universallySupported: PlatformPrimitive[];
  partiallySupported: PlatformPrimitive[];
  unsupportedAcrossAll: PlatformPrimitive[];
  providerGaps: Record<string, PlatformPrimitive[]>;
}
