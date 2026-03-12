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
import {
  renderCallLocalSupportedOptionsSection,
  renderCredentialsSection,
  renderFrameworkWiringSection,
  renderLocalCallSection,
  renderStateBackendSection,
  type PackageManager
} from "./docs/render";

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

interface DocsCheckOptions {
  config?: string;
  stage?: string;
  readme?: string;
  json?: boolean;
}

interface DocsSyncOptions {
  config?: string;
  stage?: string;
  readme?: string;
  dryRun?: boolean;
  json?: boolean;
}

async function assertReadmeExists(readmePath: string): Promise<void> {
  if (!(await pathExists(readmePath))) {
    throw new Error(`README not found at ${readmePath}`);
  }
}

function buildDocsCheckIssues(context: DocsContext, readmeContent: string): string[] {
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
  return issues;
}

function syncCallLocalOptionsSection(content: string, replacement: string): string {
  const callLocalOptionsPattern = /^Supported options for `call:local`:/m;
  if (callLocalOptionsPattern.test(content)) {
    const pattern = /^Supported options for `call:local`:[\s\S]*?(?=^## |\Z)/m;
    return content.replace(pattern, `${replacement}\n\n`);
  }
  return content.replace(/^## Credentials$/m, `${replacement}\n\n## Credentials`);
}

function applyReadmeSync(before: string, context: DocsContext): string {
  const sections = renderReadmeSections(context);
  let after = before;
  after = replaceSection(after, "Commands", sections.commands);
  after = replaceSection(after, "Local Call (Provider-mimic)", sections.localCall);
  after = replaceSection(after, "Credentials", sections.credentials);
  after = replaceSection(after, "State Backend", sections.stateBackend);
  after = syncCallLocalOptionsSection(after, sections.options);
  if (context.hasRuntimeNode) {
    return replaceSection(after, "Framework Wiring (Optional)", sections.framework);
  }
  return hasFrameworkSection(after) ? removeSection(after, "Framework Wiring (Optional)") : after;
}

async function runDocsCheck(options: DocsCheckOptions): Promise<void> {
  const context = await buildDocsContext(options);
  await assertReadmeExists(context.readmePath);
  const readmeContent = await readFile(context.readmePath, "utf8");
  const issues = buildDocsCheckIssues(context, readmeContent);
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

async function runDocsSync(options: DocsSyncOptions): Promise<void> {
  const context = await buildDocsContext(options);
  await assertReadmeExists(context.readmePath);
  const before = await readFile(context.readmePath, "utf8");
  const after = applyReadmeSync(before, context);
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
  } else if (changed && options.dryRun) {
    info(`docs sync preview: changes detected for ${context.readmePath}`);
  } else if (changed) {
    success(`docs sync wrote ${context.readmePath}`);
  } else {
    success("docs sync: README already up to date");
  }
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
    .action(async (options: DocsCheckOptions) => runDocsCheck(options));

  docs
    .command("sync")
    .description("Regenerate README command/config sections from runfabric.yml and project metadata")
    .option("-c, --config <path>", "Path to runfabric config")
    .option("-s, --stage <name>", "Stage name override")
    .option("-r, --readme <path>", "Path to README file (default: project README.md)")
    .option("--dry-run", "Preview without writing changes")
    .option("--json", "Emit JSON output")
    .action(async (options: DocsSyncOptions) => runDocsSync(options));
};
