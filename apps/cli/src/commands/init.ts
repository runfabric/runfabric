import type { CommandRegistrar } from "../types/cli";
import { mkdir, writeFile } from "node:fs/promises";
import { join, resolve } from "node:path";
import { error, info, success } from "../utils/logger";

type InitTemplateName = "api" | "worker" | "queue" | "cron";

interface InitTemplateDefinition {
  name: InitTemplateName;
  defaultService: string;
  triggerBlock: string[];
  handlerBody: string[];
}

const templateDefinitions: Record<InitTemplateName, InitTemplateDefinition> = {
  api: {
    name: "api",
    defaultService: "hello-api",
    triggerBlock: [
      "triggers:",
      "  - type: http",
      "    method: GET",
      "    path: /hello"
    ],
    handlerBody: [
      '  body: JSON.stringify({ message: "hello from api template" })'
    ]
  },
  worker: {
    name: "worker",
    defaultService: "hello-worker",
    triggerBlock: [
      "triggers:",
      "  - type: http",
      "    method: POST",
      "    path: /work"
    ],
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
    triggerBlock: [
      "triggers:",
      "  - type: queue",
      "    queue: jobs"
    ],
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
    triggerBlock: [
      "triggers:",
      "  - type: cron",
      "    schedule: \"*/5 * * * *\""
    ],
    handlerBody: [
      "  body: JSON.stringify({",
      '    message: "cron template tick",',
      "    at: new Date().toISOString()",
      "  })"
    ]
  }
};

function isTemplateName(value: string): value is InitTemplateName {
  return value === "api" || value === "worker" || value === "queue" || value === "cron";
}

function buildConfigContent(
  template: InitTemplateDefinition,
  service: string,
  provider: string
): string {
  return [
    `service: ${service}`,
    "runtime: nodejs",
    "entry: src/index.ts",
    "",
    "providers:",
    `  - ${provider}`,
    "",
    ...template.triggerBlock,
    ""
  ].join("\n");
}

function buildHandlerContent(template: InitTemplateDefinition): string {
  return [
    'import type { UniversalHandler } from "@runfabric/core";',
    "",
    "export const handler: UniversalHandler = async (req) => ({",
    "  status: 200,",
    '  headers: { "content-type": "application/json" },',
    ...template.handlerBody,
    "});",
    ""
  ].join("\n");
}

export const registerInitCommand: CommandRegistrar = (program) => {
  program
    .command("init")
    .description("Initialize a runfabric project scaffold")
    .option("--dir <path>", "Directory to initialize", ".")
    .option("--template <name>", "Template: api, worker, queue, cron", "api")
    .option("--provider <name>", "Primary provider for generated config", "aws-lambda")
    .option("--service <name>", "Service name override")
    .action(
      async (options: { dir: string; template: string; provider: string; service?: string }) => {
        if (!isTemplateName(options.template)) {
          error(`unknown template: ${options.template}`);
          error("supported templates: api, worker, queue, cron");
          process.exitCode = 1;
          return;
        }

        const template = templateDefinitions[options.template];
        const service = options.service && options.service.trim().length > 0
          ? options.service.trim()
          : template.defaultService;

        const projectDir = resolve(options.dir);
        await mkdir(join(projectDir, "src"), { recursive: true });

        const configPath = join(projectDir, "runfabric.yml");
        const handlerPath = join(projectDir, "src", "index.ts");

        await writeFile(configPath, buildConfigContent(template, service, options.provider), "utf8");
        await writeFile(handlerPath, buildHandlerContent(template), "utf8");

        info(`created ${configPath}`);
        info(`created ${handlerPath}`);
        success(`project scaffold initialized (${template.name} template)`);
      }
    );
};
