import { TriggerEnum, type ProjectConfig } from "@runfabric/core";

export type PackageManager = "npm" | "pnpm" | "yarn" | "bun";

function packageManagerAddCommand(packageManager: PackageManager, packages: string[]): string {
  if (packageManager === "pnpm") {
    return `pnpm add ${packages.join(" ")}`;
  }
  if (packageManager === "yarn") {
    return `yarn add ${packages.join(" ")}`;
  }
  if (packageManager === "bun") {
    return `bun add ${packages.join(" ")}`;
  }
  return `npm install ${packages.join(" ")}`;
}

function renderHttpLocalCallSection(
  commandPrefix: string,
  provider: string,
  trigger: ProjectConfig["triggers"][number] | undefined
): string {
  const method = typeof trigger?.method === "string" && trigger.method.trim().length > 0
    ? trigger.method.trim().toUpperCase()
    : "GET";
  const path = typeof trigger?.path === "string" && trigger.path.trim().length > 0
    ? trigger.path.trim()
    : "/hello";

  return [
    "## Local Call (Provider-mimic)",
    "",
    "Use built-in `runfabric call-local` to start a local HTTP server that forwards provider-shaped requests to your handler.",
    "",
    "```bash",
    `${commandPrefix} call:local`,
    `curl -i http://127.0.0.1:8787${path}`,
    "# stop server: Ctrl+C or type 'exit' and press Enter",
    `${commandPrefix} call:local -- --provider ${provider} --host 127.0.0.1 --port 8787 --serve --watch`,
    `${commandPrefix} call:local -- --provider ${provider} --method ${method} --path ${path}`,
    `${commandPrefix} call:local -- --provider ${provider} --event ./event.json`,
    "```"
  ].join("\n");
}

function eventSamplePayload(triggerType: TriggerEnum): string {
  switch (triggerType) {
    case TriggerEnum.Queue:
      return '{ "records": [ { "body": { "jobId": "demo-1" } } ] }';
    case TriggerEnum.Storage:
      return '{ "records": [ { "bucket": "uploads", "key": "incoming/file.jpg", "event": "ObjectCreated:Put" } ] }';
    case TriggerEnum.EventBridge:
      return '{ "source": "app.source", "detail-type": "demo.event", "detail": { "id": "1" } }';
    case TriggerEnum.PubSub:
      return '{ "subscription": "projects/demo/subscriptions/events", "message": { "data": "eyJpZCI6IjEifQ==" } }';
    case TriggerEnum.Cron:
      return '{ "source": "runfabric.dev", "detail-type": "scheduled", "time": "2026-01-01T00:00:00.000Z" }';
    case TriggerEnum.Kafka:
      return '{ "records": [ { "topic": "events", "key": "k1", "value": { "id": "1" } } ] }';
    case TriggerEnum.RabbitMq:
      return '{ "messages": [ { "routingKey": "jobs.created", "payload": { "id": "1" } } ] }';
    default:
      return '{ "event": "example" }';
  }
}

function renderEventLocalCallSection(
  commandPrefix: string,
  provider: string,
  triggerType: TriggerEnum
): string {
  const eventFileName = `event.${triggerType}.json`;
  return [
    "## Local Call (Provider-mimic)",
    "",
    `${triggerType} scaffolds are event-driven. Use \`--event\` payload simulation for local calls.`,
    "",
    `Example \`${eventFileName}\`:`,
    "",
    "```json",
    eventSamplePayload(triggerType),
    "```",
    "",
    "```bash",
    `${commandPrefix} call:local -- --provider ${provider} --event ./${eventFileName}`,
    "```"
  ].join("\n");
}

export function renderLocalCallSection(
  commandPrefix: string,
  provider: string,
  trigger: ProjectConfig["triggers"][number] | undefined
): string {
  const triggerType = trigger?.type || TriggerEnum.Http;
  if (triggerType === TriggerEnum.Http) {
    return renderHttpLocalCallSection(commandPrefix, provider, trigger);
  }
  return renderEventLocalCallSection(commandPrefix, provider, triggerType);
}

export function renderCallLocalSupportedOptionsSection(): string {
  return [
    "Supported options for `call:local`:",
    "",
    "- `--serve`",
    "- `--watch`",
    "- `--host <host>`",
    "- `--port <number>`",
    "- `--provider <id>`",
    "- `--method <HTTP_METHOD>`",
    "- `--path </route>`",
    "- `--query key=value&key2=value2`",
    "- `--header key:value` (repeatable)",
    "- `--body <string>`",
    "- `--event <path-to-json>`"
  ].join("\n");
}

export function renderFrameworkWiringSection(packageManager: PackageManager): string {
  const runtimeInstall = packageManagerAddCommand(packageManager, ["@runfabric/runtime-node"]);
  const expressInstall = packageManagerAddCommand(packageManager, ["express"]);
  const fastifyInstall = packageManagerAddCommand(packageManager, ["fastify"]);
  const nestInstall = packageManagerAddCommand(packageManager, [
    "@nestjs/core",
    "@nestjs/common",
    "@nestjs/platform-express",
    "reflect-metadata",
    "rxjs"
  ]);

  return [
    "## Framework Wiring (Optional)",
    "",
    "If you convert this scaffold to Express/Fastify/Nest, use the runtime wrapper:",
    "",
    "```bash",
    runtimeInstall,
    `${expressInstall}   # optional`,
    `${fastifyInstall}   # optional`,
    `${nestInstall}   # optional`,
    "```",
    "",
    "```ts",
    'import type { UniversalHandler } from "@runfabric/core";',
    'import { createHandler } from "@runfabric/runtime-node";',
    "",
    "export const handler: UniversalHandler = createHandler(appOrFastifyOrNestApp);",
    "```",
    "",
    "Nest TypeScript projects must enable `experimentalDecorators` and `emitDecoratorMetadata` in `tsconfig.json`."
  ].join("\n");
}

export function renderCredentialsSection(
  commandPrefix: string,
  provider: string,
  credentialEnvVars: string[]
): string {
  const credentialLines =
    credentialEnvVars.length > 0
      ? credentialEnvVars.map((envName) => `export ${envName}="your-value"`)
      : ['# no provider credential schema exposed for this provider'];
  const envFileLines =
    credentialEnvVars.length > 0
      ? credentialEnvVars.map((envName) => `${envName}=your-value`)
      : ["# credentials"];

  return [
    "## Credentials",
    "",
    `Set credentials for \`${provider}\` in your shell before running deploy:`,
    "",
    "```bash",
    ...credentialLines,
    "```",
    "",
    "Or create `.env` in this project root:",
    "",
    "```dotenv",
    ...envFileLines,
    "```",
    "",
    "Load `.env` and run:",
    "",
    "```bash",
    "set -a",
    "source .env",
    "set +a",
    `${commandPrefix} deploy`,
    "```",
    "",
    "Credential quick matrix (providers + state backends): https://github.com/runfabric/runfabric/blob/main/docs/CREDENTIALS_MATRIX.md"
  ].join("\n");
}

export function renderStateBackendSection(backend: string): string {
  const hints = (() => {
    if (backend === "local") {
      return ["# local backend selected; no additional state credentials required"];
    }
    if (backend === "postgres") {
      return ['RUNFABRIC_STATE_POSTGRES_URL="postgres://user:pass@host:5432/dbname?sslmode=require"'];
    }
    if (backend === "s3") {
      return [
        'RUNFABRIC_STATE_S3_BUCKET="your-state-bucket"',
        'AWS_REGION="us-east-1"',
        'AWS_ACCESS_KEY_ID="your-key"',
        'AWS_SECRET_ACCESS_KEY="your-secret"'
      ];
    }
    if (backend === "gcs") {
      return [
        'RUNFABRIC_STATE_GCS_BUCKET="your-state-bucket"',
        'GOOGLE_APPLICATION_CREDENTIALS="/path/to/service-account.json"'
      ];
    }
    if (backend === "azblob") {
      return [
        'RUNFABRIC_STATE_AZBLOB_CONTAINER="runfabric-state"',
        'AZURE_STORAGE_CONNECTION_STRING="your-connection-string"'
      ];
    }
    return ["# set backend-specific credentials"];
  })();

  return [
    "## State Backend",
    "",
    `Configured state backend in \`runfabric.yml\`: \`${backend}\`.`,
    "",
    "Typical environment variables for this backend:",
    "",
    "```bash",
    ...hints,
    "```"
  ].join("\n");
}
