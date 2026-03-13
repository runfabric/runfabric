import { readFile } from "node:fs/promises";
import { dirname, resolve } from "node:path";
import { parseYaml } from "@runfabric/planner";

export interface ComposeServiceConfig {
  name: string;
  config: string;
  dependsOn: string[];
}

export interface ComposeConfig {
  services: ComposeServiceConfig[];
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

export async function loadComposeConfig(composePath: string): Promise<ComposeConfig> {
  const absolutePath = resolve(composePath);
  const baseDir = dirname(absolutePath);
  const fileContent = await readFile(absolutePath, "utf8");
  const parsed = parseYaml(fileContent);
  const readDependsOn = (value: unknown, index: number): string[] => {
    if (value === undefined) {
      return [];
    }
    if (!Array.isArray(value)) {
      throw new Error(`services[${index}].dependsOn must be an array`);
    }
    return value.map((item, itemIndex) => {
      if (typeof item !== "string" || item.trim().length === 0) {
        throw new Error(`services[${index}].dependsOn[${itemIndex}] must be a non-empty string`);
      }
      return item.trim();
    });
  };
  if (!isRecord(parsed)) {
    throw new Error("Invalid compose file: root must be an object");
  }
  if (!Array.isArray(parsed.services)) {
    throw new Error("Invalid compose file: services must be an array");
  }
  const services: ComposeServiceConfig[] = parsed.services.map((service, index) => {
    if (!isRecord(service)) {
      throw new Error(`Invalid compose file: services[${index}] must be an object`);
    }
    if (typeof service.name !== "string" || service.name.trim().length === 0) {
      throw new Error(`Invalid compose file: services[${index}].name must be a non-empty string`);
    }
    if (typeof service.config !== "string" || service.config.trim().length === 0) {
      throw new Error(`Invalid compose file: services[${index}].config must be a non-empty string`);
    }
    return {
      name: service.name.trim(),
      config: resolve(baseDir, service.config.trim()),
      dependsOn: readDependsOn(service.dependsOn, index)
    };
  });
  const uniqueNames = new Set<string>();
  for (const service of services) {
    if (uniqueNames.has(service.name)) {
      throw new Error(`Invalid compose file: duplicate service name ${service.name}`);
    }
    uniqueNames.add(service.name);
  }
  return { services };
}

export function sortComposeServices(config: ComposeConfig): ComposeServiceConfig[] {
  const byName = new Map(config.services.map((service) => [service.name, service]));
  const ordered: ComposeServiceConfig[] = [];
  const visiting = new Set<string>();
  const visited = new Set<string>();

  function visit(name: string): void {
    if (visited.has(name)) {
      return;
    }
    if (visiting.has(name)) {
      throw new Error(`compose dependency cycle detected at ${name}`);
    }
    const service = byName.get(name);
    if (!service) {
      throw new Error(`compose dependency references missing service: ${name}`);
    }

    visiting.add(name);
    for (const dependency of service.dependsOn) {
      if (!byName.has(dependency)) {
        throw new Error(`compose service ${name} depends on unknown service ${dependency}`);
      }
      visit(dependency);
    }
    visiting.delete(name);
    visited.add(name);
    ordered.push(service);
  }

  for (const service of config.services) {
    visit(service.name);
  }

  return ordered;
}

export function composeServiceLevels(config: ComposeConfig): ComposeServiceConfig[][] {
  const byName = new Map(config.services.map((service) => [service.name, service]));
  const indexByName = new Map(config.services.map((service, index) => [service.name, index]));
  const indegree = new Map<string, number>();
  const dependents = new Map<string, string[]>();
  for (const service of config.services) {
    indegree.set(service.name, service.dependsOn.length);
    for (const dependency of service.dependsOn) {
      if (!byName.has(dependency)) {
        throw new Error(`compose service ${service.name} depends on unknown service ${dependency}`);
      }
      const items = dependents.get(dependency) || [];
      items.push(service.name);
      dependents.set(dependency, items);
    }
  }
  const levels: ComposeServiceConfig[][] = [];
  let frontier = config.services.filter((service) => (indegree.get(service.name) || 0) === 0);
  let visitedCount = 0;
  while (frontier.length > 0) {
    const currentLevel = [...frontier];
    levels.push(currentLevel);
    visitedCount += currentLevel.length;
    const nextNames: string[] = [];
    for (const service of currentLevel) {
      const downstream = dependents.get(service.name) || [];
      for (const childName of downstream) {
        const nextDegree = (indegree.get(childName) || 0) - 1;
        indegree.set(childName, nextDegree);
        if (nextDegree === 0) {
          nextNames.push(childName);
        }
      }
    }
    frontier = nextNames
      .sort((left, right) => (indexByName.get(left) || 0) - (indexByName.get(right) || 0))
      .map((name) => byName.get(name))
      .filter((value): value is ComposeServiceConfig => Boolean(value));
  }
  if (visitedCount !== config.services.length) {
    const unresolved = config.services.find((service) => (indegree.get(service.name) || 0) > 0);
    throw new Error(`compose dependency cycle detected at ${unresolved?.name || "unknown service"}`);
  }
  return levels;
}

export function toComposeOutputEnvKey(service: string, provider: string): string {
  const normalizedService = service.toUpperCase().replace(/[^A-Z0-9]/g, "_");
  const normalizedProvider = provider.toUpperCase().replace(/[^A-Z0-9]/g, "_");
  return `RUNFABRIC_OUTPUT_${normalizedService}_${normalizedProvider}_ENDPOINT`;
}
