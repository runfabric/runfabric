import { getProviderPackageName } from "../../providers/registry";
import type {
  InitLanguage,
  InitTemplateDefinition,
  PackageManager,
  StateBackend
} from "./types";
import { normalizePackageName } from "./types";

function stateBackendLines(stateBackend: StateBackend): string[] {
  if (stateBackend === "local") {
    return ["state:", "  backend: local", "  local:", "    dir: ./.runfabric/state"];
  }
  if (stateBackend === "postgres") {
    return [
      "state:",
      "  backend: postgres",
      "  postgres:",
      "    connectionStringEnv: RUNFABRIC_STATE_POSTGRES_URL",
      "    schema: public",
      "    table: runfabric_state"
    ];
  }
  if (stateBackend === "s3") {
    return [
      "state:",
      "  backend: s3",
      "  s3:",
      "    bucket: ${env:RUNFABRIC_STATE_S3_BUCKET}",
      "    region: ${env:AWS_REGION,us-east-1}",
      "    keyPrefix: runfabric/state",
      "    useLockfile: true"
    ];
  }
  if (stateBackend === "gcs") {
    return ["state:", "  backend: gcs", "  gcs:", "    bucket: ${env:RUNFABRIC_STATE_GCS_BUCKET}", "    prefix: runfabric/state"];
  }
  return ["state:", "  backend: azblob", "  azblob:", "    container: ${env:RUNFABRIC_STATE_AZBLOB_CONTAINER}", "    prefix: runfabric/state"];
}

export function buildConfigContent(
  template: InitTemplateDefinition,
  service: string,
  provider: string,
  language: InitLanguage,
  stateBackend: StateBackend
): string {
  const extension = language === "ts" ? "ts" : "js";
  return [
    `service: ${service}`,
    "runtime: nodejs",
    `entry: src/index.${extension}`,
    "",
    "providers:",
    `  - ${provider}`,
    "",
    ...stateBackendLines(stateBackend),
    "",
    ...template.triggerBlock,
    ""
  ].join("\n");
}

export function buildHandlerContent(template: InitTemplateDefinition, language: InitLanguage): string {
  if (language === "ts") {
    return [
      'import type { UniversalHandler, UniversalRequest, UniversalResponse } from "@runfabric/core";',
      "",
      "export const handler: UniversalHandler = async (",
      "  req: UniversalRequest",
      "): Promise<UniversalResponse> => ({",
      "  status: 200,",
      '  headers: { "content-type": "application/json" },',
      ...template.handlerBody,
      "});",
      ""
    ].join("\n");
  }
  return [
    "export const handler = async (req) => ({",
    "  status: 200,",
    '  headers: { "content-type": "application/json" },',
    ...template.handlerBody,
    "});",
    ""
  ].join("\n");
}

export function buildPackageJsonContent(
  service: string,
  language: InitLanguage,
  provider: string
): string {
  const scripts: Record<string, string> = {
    doctor: "runfabric doctor",
    plan: "runfabric plan",
    build: language === "ts" ? "tsc -p tsconfig.json" : "node -e \"console.log('no build step for js template')\"",
    deploy: "runfabric deploy",
    "call:local": "runfabric call-local -c runfabric.yml --serve --watch"
  };
  if (language === "ts") {
    scripts.typecheck = "tsc --noEmit -p tsconfig.json";
  }

  const providerPackage = getProviderPackageName(provider);
  const dependencies: Record<string, string> = { "@runfabric/core": "^0.1.0" };
  if (providerPackage) {
    dependencies[providerPackage] = "^0.1.0";
  }

  const packageJson: Record<string, unknown> = {
    name: normalizePackageName(service),
    private: true,
    version: "0.1.0",
    type: "module",
    scripts,
    dependencies
  };
  if (language === "ts") {
    packageJson.devDependencies = { typescript: "^5.9.2", "@types/node": "^24.5.2" };
  }
  return `${JSON.stringify(packageJson, null, 2)}\n`;
}

export function buildTsConfigContent(): string {
  return [
    "{",
    "  \"compilerOptions\": {",
    "    \"target\": \"ES2022\",",
    "    \"module\": \"NodeNext\",",
    "    \"moduleResolution\": \"NodeNext\",",
    "    \"strict\": true,",
    "    \"esModuleInterop\": true,",
    "    \"types\": [\"node\"],",
    "    \"skipLibCheck\": true,",
    "    \"outDir\": \"dist\"",
    "  },",
    "  \"include\": [\"src/**/*.ts\"]",
    "}",
    ""
  ].join("\n");
}

export function buildGitIgnoreContent(): string {
  return ["node_modules", "dist", ".runfabric", ".env", ""].join("\n");
}

function runCommandPrefix(packageManager: PackageManager): string {
  if (packageManager === "pnpm") {
    return "pnpm run";
  }
  if (packageManager === "yarn") {
    return "yarn";
  }
  if (packageManager === "bun") {
    return "bun run";
  }
  return "npm run";
}

function buildLocalCallReadmeSection(params: {
  template: InitTemplateDefinition;
  provider: string;
  commandPrefix: string;
}): string[] {
  if (params.template.name === "api" || params.template.name === "worker") {
    const path = params.template.name === "api" ? "/hello" : "/work";
    const method = params.template.name === "api" ? "GET" : "POST";
    return [
      "## Local Call (Provider-mimic)",
      "",
      "Use built-in `runfabric call-local` to start a local HTTP server that forwards provider-shaped requests to your handler.",
      "",
      "```bash",
      `${params.commandPrefix} call:local`,
      `curl -i http://127.0.0.1:8787${path}`,
      "# stop server: Ctrl+C or type 'exit' and press Enter",
      `${params.commandPrefix} call:local -- --provider ${params.provider} --host 127.0.0.1 --port 8787 --serve --watch`,
      `${params.commandPrefix} call:local -- --provider ${params.provider} --method ${method} --path ${path}`,
      `${params.commandPrefix} call:local -- --provider ${params.provider} --event ./event.json`,
      "```"
    ];
  }
  return params.template.name === "queue"
    ? buildQueueLocalCallReadmeSection(params.commandPrefix, params.provider)
    : buildCronLocalCallReadmeSection(params.commandPrefix, params.provider);
}

function buildQueueLocalCallReadmeSection(commandPrefix: string, provider: string): string[] {
  return [
    "## Local Call (Provider-mimic)",
    "",
    "Queue scaffolds are event-driven. Use `--event` payload simulation for local calls.",
    "",
    "Example `event.queue.json`:",
    "",
    "```json",
    '{ "records": [ { "body": { "jobId": "demo-1" } } ] }',
    "```",
    "",
    "```bash",
    `${commandPrefix} call:local -- --provider ${provider} --event ./event.queue.json`,
    "```"
  ];
}

function buildCronLocalCallReadmeSection(commandPrefix: string, provider: string): string[] {
  return [
    "## Local Call (Provider-mimic)",
    "",
    "Cron scaffolds are event-driven. Use `--event` payload simulation for local calls.",
    "",
    "Example `event.cron.json`:",
    "",
    "```json",
    '{ "source": "runfabric.dev", "detail-type": "scheduled", "time": "2026-01-01T00:00:00.000Z" }',
    "```",
    "",
    "```bash",
    `${commandPrefix} call:local -- --provider ${provider} --event ./event.cron.json`,
    "```"
  ];
}

export function packageManagerAddCommand(packageManager: PackageManager, packages: string[]): string {
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

function buildFrameworkWiringReadmeSection(params: {
  template: InitTemplateDefinition;
  packageManager: PackageManager;
}): string[] {
  if (params.template.name !== "api" && params.template.name !== "worker") {
    return [];
  }

  const runtimeInstall = packageManagerAddCommand(params.packageManager, ["@runfabric/runtime-node"]);
  const expressInstall = packageManagerAddCommand(params.packageManager, ["express"]);
  const fastifyInstall = packageManagerAddCommand(params.packageManager, ["fastify"]);
  const nestInstall = packageManagerAddCommand(params.packageManager, [
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
    "Nest TypeScript projects must enable `experimentalDecorators` and `emitDecoratorMetadata` in `tsconfig.json`.",
    "After trigger or framework changes, update this README command/examples section so project docs stay aligned.",
    ""
  ];
}

function stateBackendEnvHints(stateBackend: StateBackend): string[] {
  if (stateBackend === "local") {
    return ["# local backend selected; no additional state credentials required"];
  }
  if (stateBackend === "postgres") {
    return ['RUNFABRIC_STATE_POSTGRES_URL="postgres://user:pass@host:5432/dbname?sslmode=require"'];
  }
  if (stateBackend === "s3") {
    return [
      'RUNFABRIC_STATE_S3_BUCKET="your-state-bucket"',
      'AWS_REGION="us-east-1"',
      'AWS_ACCESS_KEY_ID="your-key"',
      'AWS_SECRET_ACCESS_KEY="your-secret"'
    ];
  }
  if (stateBackend === "gcs") {
    return [
      'RUNFABRIC_STATE_GCS_BUCKET="your-state-bucket"',
      'GOOGLE_APPLICATION_CREDENTIALS="/path/to/service-account.json"'
    ];
  }
  return [
    'RUNFABRIC_STATE_AZBLOB_CONTAINER="runfabric-state"',
    'AZURE_STORAGE_CONNECTION_STRING="your-connection-string"'
  ];
}

function buildReadmeCredentialsSection(
  commandPrefix: string,
  provider: string,
  credentialLines: string[],
  envFileLines: string[]
): string[] {
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
  ];
}

function buildReadmeStateBackendSection(stateBackend: StateBackend, stateHints: string[]): string[] {
  return [
    "## State Backend",
    "",
    `Configured state backend in \`runfabric.yml\`: \`${stateBackend}\`.`,
    "",
    "Typical environment variables for this backend:",
    "",
    "```bash",
    ...stateHints,
    "```"
  ];
}

function buildReadmeGeneratedFilesSection(language: InitLanguage, extension: string, skippedInstall: boolean): string[] {
  return [
    "## Generated Files",
    "",
    "- `runfabric.yml`",
    `- \`src/index.${extension}\``,
    "- `package.json`",
    "- `.gitignore`",
    ...(language === "ts" ? ["- `tsconfig.json`"] : []),
    ...(skippedInstall ? ["", "Dependencies were not installed automatically (`--skip-install` was used)."] : []),
    ""
  ];
}

function buildReadmeCommandSections(
  commandPrefix: string,
  localCallSection: string[],
  language: InitLanguage
): string[] {
  return [
    "## Commands",
    "",
    "From this project directory:",
    "",
    "```bash",
    `${commandPrefix} doctor`,
    `${commandPrefix} plan`,
    `${commandPrefix} build`,
    `${commandPrefix} deploy`,
    `${commandPrefix} call:local`,
    "```",
    "",
    ...localCallSection,
    "",
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
    "- `--event <path-to-json>`",
    "",
    language === "ts"
      ? "- TypeScript mode: if no built handler is found, `call-local` runs an initial `tsc -p tsconfig.json` automatically (requires `typescript` in dev dependencies)."
      : "- JavaScript mode: handler is loaded directly from configured entry.",
    ""
  ];
}

function buildReadmeHeaderSection(params: {
  service: string;
  template: InitTemplateDefinition;
  provider: string;
  language: InitLanguage;
}): string[] {
  return [
    `# ${params.service}`,
    "",
    `Generated by \`runfabric init\` (${params.template.name}, ${params.provider}, ${params.language}).`,
    "",
    "## Handler Import",
    "",
    "Use this in your handler file:",
    "",
    "```ts",
    params.language === "ts"
      ? 'import type { UniversalHandler, UniversalRequest, UniversalResponse } from "@runfabric/core";'
      : "// JavaScript template does not require type imports",
    "```",
    ""
  ];
}

interface ProjectReadmeContentParams {
  service: string;
  provider: string;
  language: InitLanguage;
  template: InitTemplateDefinition;
  stateBackend: StateBackend;
  packageManager: PackageManager;
  credentialEnvVars: string[];
  skippedInstall: boolean;
}

export function buildProjectReadmeContent(params: ProjectReadmeContentParams): string {
  const commandPrefix = runCommandPrefix(params.packageManager);
  const localCallSection = buildLocalCallReadmeSection({
    template: params.template,
    provider: params.provider,
    commandPrefix
  });
  const frameworkWiringSection = buildFrameworkWiringReadmeSection({
    template: params.template,
    packageManager: params.packageManager
  });
  const extension = params.language === "ts" ? "ts" : "js";
  const credentialLines =
    params.credentialEnvVars.length > 0
      ? params.credentialEnvVars.map((envName) => `export ${envName}=\"your-value\"`)
      : ['# no provider credential schema exposed for this provider'];
  const envFileLines =
    params.credentialEnvVars.length > 0
      ? params.credentialEnvVars.map((envName) => `${envName}=your-value`)
      : ["# credentials"];
  const stateHints = stateBackendEnvHints(params.stateBackend);
  const commandsSection = buildReadmeCommandSections(commandPrefix, localCallSection, params.language);
  const credentialsSection = buildReadmeCredentialsSection(
    commandPrefix,
    params.provider,
    credentialLines,
    envFileLines
  );
  const stateBackendSection = buildReadmeStateBackendSection(params.stateBackend, stateHints);
  const generatedFilesSection = buildReadmeGeneratedFilesSection(
    params.language,
    extension,
    params.skippedInstall
  );
  const headerSection = buildReadmeHeaderSection(params);

  return [
    ...headerSection,
    ...commandsSection,
    ...frameworkWiringSection,
    ...credentialsSection,
    "",
    ...stateBackendSection,
    "",
    ...generatedFilesSection
  ].join("\n");
}
