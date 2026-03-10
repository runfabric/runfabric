import type {
  PlatformPrimitive,
  PrimitiveCompatibilityReport,
  ProviderPrimitiveProfile
} from "@runfabric/core";

const allPrimitives: PlatformPrimitive[] = [
  "compute",
  "functionRuntime",
  "eventTriggers",
  "workflowOrchestration",
  "queue",
  "eventBus",
  "scheduler",
  "stateStorage",
  "observability"
];

export function createPrimitiveCompatibilityReport(
  selectedProviders: string[],
  profiles: Record<string, ProviderPrimitiveProfile | undefined>
): PrimitiveCompatibilityReport {
  const providerGaps: Record<string, PlatformPrimitive[]> = {};

  for (const provider of selectedProviders) {
    const profile = profiles[provider];
    if (!profile) {
      providerGaps[provider] = [...allPrimitives];
      continue;
    }
    providerGaps[provider] = allPrimitives.filter((primitive) => !profile[primitive]);
  }

  const universallySupported = allPrimitives.filter((primitive) =>
    selectedProviders.every((provider) => {
      const profile = profiles[provider];
      return Boolean(profile?.[primitive]);
    })
  );

  const unsupportedAcrossAll = allPrimitives.filter((primitive) =>
    selectedProviders.every((provider) => {
      const profile = profiles[provider];
      return !profile?.[primitive];
    })
  );

  const partiallySupported = allPrimitives.filter((primitive) => {
    if (universallySupported.includes(primitive)) {
      return false;
    }
    if (unsupportedAcrossAll.includes(primitive)) {
      return false;
    }
    return true;
  });

  return {
    selectedProviders,
    universallySupported,
    partiallySupported,
    unsupportedAcrossAll,
    providerGaps
  };
}
