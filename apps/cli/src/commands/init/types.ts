export type InitTemplateName = "api" | "worker" | "queue" | "cron";
export type InitLanguage = "ts" | "js";
export type PackageManager = "npm" | "pnpm" | "yarn" | "bun";
export type StateBackend = "local" | "postgres" | "s3" | "gcs" | "azblob";

export interface InitTemplateDefinition {
  name: InitTemplateName;
  defaultService: string;
  triggerBlock: string[];
  handlerBody: string[];
}

export const templateDefinitions: Record<InitTemplateName, InitTemplateDefinition> = {
  api: {
    name: "api",
    defaultService: "hello-api",
    triggerBlock: ["triggers:", "  - type: http", "    method: GET", "    path: /hello"],
    handlerBody: ['  body: JSON.stringify({ message: "hello from api template" })']
  },
  worker: {
    name: "worker",
    defaultService: "hello-worker",
    triggerBlock: ["triggers:", "  - type: http", "    method: POST", "    path: /work"],
    handlerBody: [
      "  body: JSON.stringify({",
      '    message: "worker template accepted work",',
      "    received: req.body || null",
      "  })"
    ]
  },
  queue: {
    name: "queue",
    defaultService: "hello-queue",
    triggerBlock: ["triggers:", "  - type: queue", "    queue: jobs"],
    handlerBody: [
      "  body: JSON.stringify({",
      '    message: "queue template processed message",',
      "    payload: req.body || null",
      "  })"
    ]
  },
  cron: {
    name: "cron",
    defaultService: "hello-cron",
    triggerBlock: ["triggers:", "  - type: cron", "    schedule: \"*/5 * * * *\""],
    handlerBody: [
      "  body: JSON.stringify({",
      '    message: "cron template tick",',
      "    at: new Date().toISOString()",
      "  })"
    ]
  }
};

export const initLanguages: InitLanguage[] = ["ts", "js"];
export const initStateBackends: StateBackend[] = ["local", "postgres", "s3", "gcs", "azblob"];

export function isTemplateName(value: string): value is InitTemplateName {
  return value === "api" || value === "worker" || value === "queue" || value === "cron";
}

export function isLanguage(value: string): value is InitLanguage {
  return value === "ts" || value === "js";
}

export function isPackageManager(value: string): value is PackageManager {
  return value === "npm" || value === "pnpm" || value === "yarn" || value === "bun";
}

export function isStateBackend(value: string): value is StateBackend {
  return value === "local" || value === "postgres" || value === "s3" || value === "gcs" || value === "azblob";
}

export function normalizePackageName(service: string): string {
  return service
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9-_]/g, "-")
    .replace(/--+/g, "-")
    .replace(/^-+/, "")
    .replace(/-+$/, "") || "runfabric-service";
}
