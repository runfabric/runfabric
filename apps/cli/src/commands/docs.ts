import type { CommandRegistrar } from "../types/cli";
import { access, readFile, writeFile } from "node:fs/promises";
import { constants } from "node:fs";
import { resolve } from "node:path";
import { TriggerEnum, type ProjectConfig } from "@runfabric/core";
import { createProviderRegistry } from "../providers/registry";
import { loadPlanningContext } from "../utils/load-config";
import { error, info, success, warn } from "../utils/logger";
import { printJson } from "../utils/output";
import { resolveProjectDir } from "../utils/resolve-project";

type PackageManager = "npm" | "pnpm" | "yarn" | "bun";

interface DocsContext {
  projectDir: string;
  readmePath: string;
  project: ProjectConfig;
  provider: string;
  stateBackend: string;
  packageManager: PackageManager;
  hasRuntimeNode: boolean;
  credentialEnvVars: string[];
}

async function pathExists(path: string): Promise<boolean> {
  try {
    await access(path, constants.F_OK);
    return true;
  } catch {
    return false;
  }
}

function normalizePackageManager(value: string | undefined): PackageManager {
  const normalized = (value || "").toLowerCase();
  if (normalized.startsWith("pnpm@")) {
    return "pnpm";
  }
  if (normalized.startsWith("yarn@")) {
    return "yarn";
  }
  if (normalized.startsWith("bun@")) {
    return "bun";
  }
  return "npm";
}

async function detectProjectPackageManager(projectDir: string): Promise<PackageManager> {
  const packageJsonPath = resolve(projectDir, "package.json");
  if (await pathExists(packageJsonPath)) {
    try {
      const packageJson = JSON.parse(await readFile(packageJsonPath, "utf8")) as {
        packageManager?: string;
      };
      const detected = normalizePackageManager(packageJson.packageManager);
      if (detected !== "npm" || packageJson.packageManager) {
        return detected;
      }
    } catch {
      // fall back to lock-file detection
    }
  }

  if (await pathExists(resolve(projectDir, "pnpm-lock.yaml"))) {
    return "pnpm";
  }
  if (await pathExists(resolve(projectDir, "yarn.lock"))) {
    return "yarn";
  }
  if (await pathExists(resolve(projectDir, "bun.lockb"))) {
    return "bun";
  }
  return "npm";
}

async function readProjectPackageJson(projectDir: string): Promise<Record<string, unknown>> {
  const packageJsonPath = resolve(projectDir, "package.json");
  if (!(await pathExists(packageJsonPath))) {
    return {};
  }
  try {
    return JSON.parse(await readFile(packageJsonPath, "utf8")) as Record<string, unknown>;
  } catch {
    return {};
  }
}

function hasDependency(packageJson: Record<string, unknown>, dependencyName: string): boolean {
  const sections = ["dependencies", "devDependencies", "peerDependencies", "optionalDependencies"] as const;
  for (const section of sections) {
    const value = packageJson[section];
    if (!value || typeof value !== "object") {
      continue;
    }
    if (dependencyName in (value as Record<string, unknown>)) {
      return true;
    }
  }
  return false;
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

function firstProvider(project: ProjectConfig): string {
  return project.providers[0] || "aws-lambda";
}

function stateBackend(project: ProjectConfig): string {
  return project.state?.backend || "local";
}

function firstTrigger(project: ProjectConfig): ProjectConfig["triggers"][number] | undefined {
  return project.triggers[0];
}

function resolveCredentialEnvVars(
  projectDir: string,
  provider: string
): string[] {
  const registry = createProviderRegistry(projectDir, [provider]);
  const adapter = registry[provider];
  const schema = adapter?.getCredentialSchema?.();
  if (!schema) {
    return [];
  }
  return schema.fields
    .map((field) => field.env.trim())
    .filter((value) => value.length > 0);
}

function renderCommandsSection(commandPrefix: string): string {
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
    "```"
  ].join("\n");
}

function renderLocalCallSection(
  commandPrefix: string,
  provider: string,
  trigger: ProjectConfig["triggers"][number] | undefined
): string {
  const triggerType = trigger?.type || TriggerEnum.Http;
  if (triggerType === TriggerEnum.Http) {
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

  const eventFileName = `event.${triggerType}.json`;
  const samplePayload = (() => {
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
  })();

  return [
    "## Local Call (Provider-mimic)",
    "",
    `${triggerType} scaffolds are event-driven. Use \`--event\` payload simulation for local calls.`,
    "",
    `Example \`${eventFileName}\`:`,
    "",
    "```json",
    samplePayload,
    "```",
    "",
    "```bash",
    `${commandPrefix} call:local -- --provider ${provider} --event ./${eventFileName}`,
    "```"
  ].join("\n");
}

function renderCallLocalSupportedOptionsSection(): string {
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

function renderFrameworkWiringSection(packageManager: PackageManager): string {
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

function renderCredentialsSection(
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

function renderStateBackendSection(backend: string): string {
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

function escapeRegex(value: string): string {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

function replaceSection(content: string, heading: string, replacementSection: string): string {
  const headingMarker = `## ${heading}`;
  const pattern = new RegExp(`^${escapeRegex(headingMarker)}\\n[\\s\\S]*?(?=^## |\\Z)`, "m");
  const normalized = replacementSection.trimEnd();
  if (pattern.test(content)) {
    return content.replace(pattern, `${normalized}\n\n`);
  }
  return `${content.trimEnd()}\n\n${normalized}\n`;
}

function removeSection(content: string, heading: string): string {
  const headingMarker = `## ${heading}`;
  const pattern = new RegExp(`^${escapeRegex(headingMarker)}\\n[\\s\\S]*?(?=^## |\\Z)`, "m");
  return content.replace(pattern, "").replace(/\n{3,}/g, "\n\n").trimEnd() + "\n";
}

function renderReadmeSections(context: DocsContext): {
  commands: string;
  localCall: string;
  options: string;
  framework: string;
  credentials: string;
  stateBackend: string;
} {
  const trigger = firstTrigger(context.project);
  const commandPrefix = runCommandPrefix(context.packageManager);

  return {
    commands: renderCommandsSection(commandPrefix),
    localCall: renderLocalCallSection(commandPrefix, context.provider, trigger),
    options: renderCallLocalSupportedOptionsSection(),
    framework: renderFrameworkWiringSection(context.packageManager),
    credentials: renderCredentialsSection(commandPrefix, context.provider, context.credentialEnvVars),
    stateBackend: renderStateBackendSection(context.stateBackend)
  };
}

function hasFrameworkSection(readmeContent: string): boolean {
  return /^## Framework Wiring \(Optional\)$/m.test(readmeContent);
}

function validateTriggerExamples(readmeContent: string, project: ProjectConfig): string[] {
  const trigger = firstTrigger(project);
  if (!trigger) {
    return ["runfabric.yml has no triggers; cannot validate README local-call examples"];
  }

  if (trigger.type === TriggerEnum.Http) {
    const method = typeof trigger.method === "string" && trigger.method.trim().length > 0
      ? trigger.method.trim().toUpperCase()
      : "GET";
    const path = typeof trigger.path === "string" && trigger.path.trim().length > 0
      ? trigger.path.trim()
      : "/hello";
    const issues: string[] = [];
    if (!readmeContent.includes(`--method ${method}`)) {
      issues.push(`README local-call section is missing expected HTTP method example (--method ${method})`);
    }
    if (!readmeContent.includes(`--path ${path}`)) {
      issues.push(`README local-call section is missing expected HTTP path example (--path ${path})`);
    }
    return issues;
  }

  const expectedEventFile = `event.${trigger.type}.json`;
  if (!readmeContent.includes(expectedEventFile)) {
    return [`README local-call section is missing expected event example file (${expectedEventFile})`];
  }
  return [];
}

async function buildDocsContext(options: {
  config?: string;
  stage?: string;
  readme?: string;
}): Promise<DocsContext> {
  const projectDir = await resolveProjectDir(process.cwd(), options.config);
  const configPath = options.config ? resolve(process.cwd(), options.config) : undefined;
  const context = await loadPlanningContext(projectDir, configPath, options.stage);
  const provider = firstProvider(context.project);
  const backend = stateBackend(context.project);
  const packageManager = await detectProjectPackageManager(projectDir);
  const packageJson = await readProjectPackageJson(projectDir);
  const hasRuntimeNode = hasDependency(packageJson, "@runfabric/runtime-node");
  const credentialEnvVars = resolveCredentialEnvVars(projectDir, provider);
  const readmePath = options.readme
    ? resolve(process.cwd(), options.readme)
    : resolve(projectDir, "README.md");

  return {
    projectDir,
    readmePath,
    project: context.project,
    provider,
    stateBackend: backend,
    packageManager,
    hasRuntimeNode,
    credentialEnvVars
  };
}

export const registerDocsCommand: CommandRegistrar = (program) => {
  const docs = program.command("docs").description("Scaffold README drift checks and sync tools");

  docs
    .command("check")
    .description("Validate README consistency against runfabric.yml and installed framework wiring")
    .option("-c, --config <path>", "Path to runfabric config")
    .option("-s, --stage <name>", "Stage name override")
    .option("-r, --readme <path>", "Path to README file (default: project README.md)")
    .option("--json", "Emit JSON output")
    .action(
      async (options: { config?: string; stage?: string; readme?: string; json?: boolean }) => {
        const context = await buildDocsContext(options);
        if (!(await pathExists(context.readmePath))) {
          throw new Error(`README not found at ${context.readmePath}`);
        }

        const readmeContent = await readFile(context.readmePath, "utf8");
        const issues: string[] = [];

        const providerLine = `Set credentials for \`${context.provider}\` in your shell before running deploy:`;
        if (!readmeContent.includes(providerLine)) {
          issues.push(`README credentials section provider mismatch or missing line: ${providerLine}`);
        }

        const backendLine = `Configured state backend in \`runfabric.yml\`: \`${context.stateBackend}\`.`;
        if (!readmeContent.includes(backendLine)) {
          issues.push(`README state-backend line mismatch or missing line: ${backendLine}`);
        }

        issues.push(...validateTriggerExamples(readmeContent, context.project));

        if (context.hasRuntimeNode && !hasFrameworkSection(readmeContent)) {
          issues.push("README missing `## Framework Wiring (Optional)` while @runfabric/runtime-node is installed");
        }

        const payload = {
          ok: issues.length === 0,
          projectDir: context.projectDir,
          readmePath: context.readmePath,
          provider: context.provider,
          stateBackend: context.stateBackend,
          triggerType: firstTrigger(context.project)?.type || null,
          hasRuntimeNode: context.hasRuntimeNode,
          issues
        };

        if (options.json) {
          printJson(payload);
        } else if (payload.ok) {
          success("docs check passed");
        } else {
          warn(`docs check found ${issues.length} issue(s)`);
          for (const issue of issues) {
            error(`- ${issue}`);
          }
        }

        if (!payload.ok) {
          process.exitCode = 1;
        }
      }
    );

  docs
    .command("sync")
    .description("Regenerate README command/config sections from runfabric.yml and project metadata")
    .option("-c, --config <path>", "Path to runfabric config")
    .option("-s, --stage <name>", "Stage name override")
    .option("-r, --readme <path>", "Path to README file (default: project README.md)")
    .option("--dry-run", "Preview without writing changes")
    .option("--json", "Emit JSON output")
    .action(
      async (options: {
        config?: string;
        stage?: string;
        readme?: string;
        dryRun?: boolean;
        json?: boolean;
      }) => {
        const context = await buildDocsContext(options);
        if (!(await pathExists(context.readmePath))) {
          throw new Error(`README not found at ${context.readmePath}`);
        }

        const before = await readFile(context.readmePath, "utf8");
        const sections = renderReadmeSections(context);
        let after = before;
        after = replaceSection(after, "Commands", sections.commands);
        after = replaceSection(after, "Local Call (Provider-mimic)", sections.localCall);
        after = replaceSection(after, "Credentials", sections.credentials);
        after = replaceSection(after, "State Backend", sections.stateBackend);

        const callLocalOptionsPattern = /^Supported options for `call:local`:/m;
        if (callLocalOptionsPattern.test(after)) {
          const pattern = /^Supported options for `call:local`:[\s\S]*?(?=^## |\Z)/m;
          after = after.replace(pattern, `${sections.options}\n\n`);
        } else {
          after = after.replace(/^## Credentials$/m, `${sections.options}\n\n## Credentials`);
        }

        if (context.hasRuntimeNode) {
          after = replaceSection(after, "Framework Wiring (Optional)", sections.framework);
        } else if (hasFrameworkSection(after)) {
          after = removeSection(after, "Framework Wiring (Optional)");
        }

        const changed = after !== before;
        if (changed && !options.dryRun) {
          await writeFile(context.readmePath, after, "utf8");
        }

        const payload = {
          ok: true,
          changed,
          dryRun: Boolean(options.dryRun),
          projectDir: context.projectDir,
          readmePath: context.readmePath,
          provider: context.provider,
          stateBackend: context.stateBackend,
          triggerType: firstTrigger(context.project)?.type || null,
          hasRuntimeNode: context.hasRuntimeNode
        };

        if (options.json) {
          printJson(payload);
        } else if (changed) {
          if (options.dryRun) {
            info(`docs sync preview: changes detected for ${context.readmePath}`);
          } else {
            success(`docs sync wrote ${context.readmePath}`);
          }
        } else {
          success("docs sync: README already up to date");
        }
      }
    );
};
