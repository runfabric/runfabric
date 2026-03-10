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

function readStringArray(value: unknown, path: string): string[] {
  if (value === undefined) {
    return [];
  }
  if (!Array.isArray(value)) {
    throw new Error(`${path} must be an array`);
  }
  return value.map((item, index) => {
    if (typeof item !== "string" || item.trim().length === 0) {
      throw new Error(`${path}[${index}] must be a non-empty string`);
    }
    return item.trim();
  });
}

export async function loadComposeConfig(composePath: string): Promise<ComposeConfig> {
  const absolutePath = resolve(composePath);
  const baseDir = dirname(absolutePath);
  const fileContent = await readFile(absolutePath, "utf8");
  const parsed = parseYaml(fileContent);

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
      dependsOn: readStringArray(service.dependsOn, `services[${index}].dependsOn`)
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

export function toComposeOutputEnvKey(service: string, provider: string): string {
  const normalizedService = service.toUpperCase().replace(/[^A-Z0-9]/g, "_");
  const normalizedProvider = provider.toUpperCase().replace(/[^A-Z0-9]/g, "_");
  return `RUNFABRIC_OUTPUT_${normalizedService}_${normalizedProvider}_ENDPOINT`;
}

