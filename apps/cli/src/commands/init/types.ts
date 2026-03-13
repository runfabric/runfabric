export type InitTemplateName =
  | "api"
  | "worker"
  | "queue"
  | "cron"
  | "storage"
  | "eventbridge"
  | "pubsub"
  | "kafka"
  | "rabbitmq";
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
  },
  storage: {
    name: "storage",
    defaultService: "hello-storage",
    triggerBlock: [
      "triggers:",
      "  - type: storage",
      "    bucket: uploads",
      "    events:",
      "      - s3:ObjectCreated:*"
    ],
    handlerBody: [
      "  body: JSON.stringify({",
      '    message: "storage template processed event",',
      "    event: req.body || null",
      "  })"
    ]
  },
  eventbridge: {
    name: "eventbridge",
    defaultService: "hello-eventbridge",
    triggerBlock: [
      "triggers:",
      "  - type: eventbridge",
      "    bus: default",
      "    pattern:",
      "      source:",
      "        - com.example.orders"
    ],
    handlerBody: [
      "  body: JSON.stringify({",
      '    message: "eventbridge template processed event",',
      "    event: req.body || null",
      "  })"
    ]
  },
  pubsub: {
    name: "pubsub",
    defaultService: "hello-pubsub",
    triggerBlock: ["triggers:", "  - type: pubsub", "    topic: events"],
    handlerBody: [
      "  body: JSON.stringify({",
      '    message: "pubsub template processed message",',
      "    event: req.body || null",
      "  })"
    ]
  },
  kafka: {
    name: "kafka",
    defaultService: "hello-kafka",
    triggerBlock: [
      "triggers:",
      "  - type: kafka",
      "    brokers:",
      "      - localhost:9092",
      "    topic: events"
    ],
    handlerBody: [
      "  body: JSON.stringify({",
      '    message: "kafka template processed message",',
      "    event: req.body || null",
      "  })"
    ]
  },
  rabbitmq: {
    name: "rabbitmq",
    defaultService: "hello-rabbitmq",
    triggerBlock: ["triggers:", "  - type: rabbitmq", "    queue: jobs", "    routingKey: jobs.created"],
    handlerBody: [
      "  body: JSON.stringify({",
      '    message: "rabbitmq template processed message",',
      "    event: req.body || null",
      "  })"
    ]
  }
};

export const initLanguages: InitLanguage[] = ["ts", "js"];
export const initStateBackends: StateBackend[] = ["local", "postgres", "s3", "gcs", "azblob"];

export function isTemplateName(value: string): value is InitTemplateName {
  return (
    value === "api" ||
    value === "worker" ||
    value === "queue" ||
    value === "cron" ||
    value === "storage" ||
    value === "eventbridge" ||
    value === "pubsub" ||
    value === "kafka" ||
    value === "rabbitmq"
  );
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
