import type { CommandRegistrar } from "../types/cli";
import { randomBytes } from "node:crypto";
import { mkdir, writeFile } from "node:fs/promises";
import { spawn } from "node:child_process";
import { basename, join, resolve } from "node:path";
import { PROVIDER_IDS } from "@runfabric/core";
import { createProviderRegistry, getProviderPackageName } from "../providers/registry";
import { error, info, success, warn } from "../utils/logger";
import { canPromptInteractively, promptSelection } from "./init/prompt";
import {
  languagePromptOptions,
  providerPromptOptions,
  stateBackendPromptOptions,
  templatePromptOptions
} from "./init/prompt-options";
import { isTemplateSupportedByProvider, supportedTemplatesForProvider } from "./init/template-support";
import {
  buildConfigContent,
  buildEnvExampleContent,
  buildGitIgnoreContent,
  buildHandlerContent,
  buildPackageJsonContent,
  buildProjectReadmeContent,
  buildTsConfigContent,
  packageManagerAddCommand
} from "./init/render";
import {
  initLanguages,
  initStateBackends,
  isLanguage,
  isPackageManager,
  isStateBackend,
  isTemplateName,
  normalizePackageName,
  templateDefinitions,
  type InitLanguage,
  type InitTemplateName,
  type PackageManager,
  type StateBackend
} from "./init/types";

interface InitCommandOptions {
  dir: string;
  template?: string;
  provider?: string;
  stateBackend?: string;
  lang?: string;
  pm?: string;
  service?: string;
  skipInstall?: boolean;
  callLocal?: boolean;
  interactive?: boolean;
}

interface InitSelections {
  templateName: InitTemplateName;
  provider: string;
  language: InitLanguage;
  stateBackend: StateBackend;
  packageManager: PackageManager;
}

interface InitFilePaths {
  configPath: string;
  handlerPath: string;
  packageJsonPath: string;
  gitIgnorePath: string;
  envExamplePath: string;
  tsConfigPath: string;
  readmePath: string;
}

function detectPackageManager(): PackageManager {
  const userAgent = process.env.npm_config_user_agent?.toLowerCase() || "";
  if (userAgent.includes("pnpm")) {
    return "pnpm";
  }
  if (userAgent.includes("yarn")) {
    return "yarn";
  }
  if (userAgent.includes("bun")) {
    return "bun";
  }
  return "npm";
}

function reportUnsupported(value: string, label: string, supported: string[]): null {
  error(`unknown ${label}: ${value}`);
  error(`supported ${label}s: ${supported.join(", ")}`);
  process.exitCode = 1;
  return null;
}

function reportUnsupportedTemplateForProvider(template: InitTemplateName, provider: string): null {
  error(`template "${template}" is not supported by provider "${provider}"`);
  const supported = supportedTemplatesForProvider(provider);
  error(`supported templates for ${provider}: ${supported.join(", ")}`);
  process.exitCode = 1;
  return null;
}

function reportNoTemplatesForProvider(provider: string): null {
  error(`no init templates are supported by provider "${provider}"`);
  process.exitCode = 1;
  return null;
}

function defaultTemplateForProvider(provider: string): InitTemplateName {
  const supported = supportedTemplatesForProvider(provider);
  if (supported.length === 0) {
    return "api";
  }
  return supported.includes("api") ? "api" : supported[0];
}

function validateTemplateSupport(template: InitTemplateName, provider: string): InitTemplateName | null {
  if (!isTemplateSupportedByProvider(template, provider)) {
    return reportUnsupportedTemplateForProvider(template, provider);
  }
  return template;
}

async function resolveTemplateName(
  options: InitCommandOptions,
  interactiveMode: boolean,
  provider: string
): Promise<InitTemplateName | null> {
  const supportedTemplates = supportedTemplatesForProvider(provider);
  if (supportedTemplates.length === 0) {
    return reportNoTemplatesForProvider(provider);
  }
  const defaultTemplate = defaultTemplateForProvider(provider);
  const value = options.template
    ? options.template
    : interactiveMode
      ? await promptSelection("Select template", templatePromptOptions(supportedTemplates), defaultTemplate)
      : defaultTemplate;
  if (!isTemplateName(value)) {
    return reportUnsupported(value, "template", ["api", "worker", "queue", "cron"]);
  }
  return validateTemplateSupport(value, provider);
}

async function resolveProviderName(options: InitCommandOptions, interactiveMode: boolean): Promise<string | null> {
  const value = options.provider
    ? options.provider
    : interactiveMode
      ? await promptSelection("Select provider", providerPromptOptions(), "aws-lambda")
      : "aws-lambda";
  if (!PROVIDER_IDS.includes(value as (typeof PROVIDER_IDS)[number])) {
    return reportUnsupported(value, "provider", [...PROVIDER_IDS]);
  }
  return value;
}

async function resolveLanguage(options: InitCommandOptions, interactiveMode: boolean): Promise<InitLanguage | null> {
  const value = options.lang
    ? options.lang
    : interactiveMode
      ? await promptSelection("Select language", languagePromptOptions(), "ts")
      : "ts";
  if (!isLanguage(value)) {
    return reportUnsupported(value, "language", ["ts", "js"]);
  }
  return value;
}

async function resolveStateBackend(options: InitCommandOptions, interactiveMode: boolean): Promise<StateBackend | null> {
  const value = options.stateBackend
    ? options.stateBackend
    : interactiveMode
      ? await promptSelection("Select state backend", stateBackendPromptOptions(), "local")
      : "local";
  if (!isStateBackend(value)) {
    return reportUnsupported(value, "state backend", [...initStateBackends]);
  }
  return value;
}

function resolvePackageManager(value: string): PackageManager | null {
  if (!isPackageManager(value)) {
    return reportUnsupported(value, "package manager", ["npm", "pnpm", "yarn", "bun"]);
  }
  return value;
}

async function resolveSelections(options: InitCommandOptions): Promise<InitSelections | null> {
  const interactiveMode = options.interactive !== false && canPromptInteractively();
  const provider = await resolveProviderName(options, interactiveMode);
  if (!provider) {
    return null;
  }
  const templateName = await resolveTemplateName(options, interactiveMode, provider);
  if (!templateName) {
    return null;
  }
  const language = await resolveLanguage(options, interactiveMode);
  if (!language) {
    return null;
  }
  const stateBackend = await resolveStateBackend(options, interactiveMode);
  if (!stateBackend) {
    return null;
  }
  const packageManager = resolvePackageManager(options.pm || detectPackageManager());
  if (!packageManager) {
    return null;
  }
  return { templateName, provider, language, stateBackend, packageManager };
}

function initFilePaths(projectDir: string, extension: string): InitFilePaths {
  return {
    configPath: join(projectDir, "runfabric.yml"),
    handlerPath: join(projectDir, "src", `index.${extension}`),
    packageJsonPath: join(projectDir, "package.json"),
    gitIgnorePath: join(projectDir, ".gitignore"),
    envExamplePath: join(projectDir, ".env.example"),
    tsConfigPath: join(projectDir, "tsconfig.json"),
    readmePath: join(projectDir, "README.md")
  };
}

function resolveCredentialEnvVars(projectDir: string, provider: string): string[] {
  const adapter = createProviderRegistry(projectDir, [provider])[provider];
  const schema = adapter?.getCredentialSchema?.();
  const values = schema?.fields.map((field) => field.env).filter((value) => value.trim().length > 0) || [];
  return [...new Set(values)];
}

function initConfigContent(
  params: {
    service: string;
    provider: string;
    language: InitLanguage;
    stateBackend: StateBackend;
    stateNamespaceId: string;
  },
  template: (typeof templateDefinitions)[InitTemplateName]
): string {
  return buildConfigContent(
    template,
    params.service,
    params.provider,
    params.language,
    params.stateBackend,
    params.stateNamespaceId
  );
}

function initReadmeContent(
  params: {
    service: string;
    provider: string;
    language: InitLanguage;
    stateBackend: StateBackend;
    packageManager: PackageManager;
    credentialEnvVars: string[];
    skipInstall: boolean;
  },
  template: (typeof templateDefinitions)[InitTemplateName]
): string {
  return buildProjectReadmeContent({
    service: params.service,
    provider: params.provider,
    language: params.language,
    template,
    stateBackend: params.stateBackend,
    packageManager: params.packageManager,
    credentialEnvVars: params.credentialEnvVars,
    skippedInstall: params.skipInstall
  });
}

async function writeProjectFiles(params: {
  projectDir: string;
  service: string;
  stateNamespaceId: string;
  provider: string;
  language: InitLanguage;
  stateBackend: StateBackend;
  packageManager: PackageManager;
  credentialEnvVars: string[];
  skipInstall: boolean;
  templateName: InitTemplateName;
}): Promise<InitFilePaths> {
  const template = templateDefinitions[params.templateName];
  const extension = params.language === "ts" ? "ts" : "js";
  const paths = initFilePaths(params.projectDir, extension);

  await mkdir(join(params.projectDir, "src"), { recursive: true });
  await writeFile(paths.configPath, initConfigContent(params, template), "utf8");
  await writeFile(paths.handlerPath, buildHandlerContent(template, params.language), "utf8");
  await writeFile(paths.packageJsonPath, buildPackageJsonContent(params.service, params.language, params.provider), "utf8");
  await writeFile(paths.gitIgnorePath, buildGitIgnoreContent(), "utf8");
  await writeFile(paths.envExamplePath, buildEnvExampleContent(params.credentialEnvVars, params.stateBackend), "utf8");
  await writeFile(paths.readmePath, initReadmeContent(params, template), "utf8");
  if (params.language === "ts") {
    await writeFile(paths.tsConfigPath, buildTsConfigContent(), "utf8");
  }

  return paths;
}

function logCreatedFiles(paths: InitFilePaths, language: InitLanguage): void {
  info(`created ${paths.configPath}`);
  info(`created ${paths.handlerPath}`);
  info(`created ${paths.packageJsonPath}`);
  info(`created ${paths.gitIgnorePath}`);
  info(`created ${paths.envExamplePath}`);
  info(`created ${paths.readmePath}`);
  if (language === "ts") {
    info(`created ${paths.tsConfigPath}`);
  }
}

async function runCommand(command: string, args: string[], cwd: string): Promise<void> {
  await new Promise<void>((resolvePromise, rejectPromise) => {
    const child = spawn(command, args, { cwd, stdio: "inherit", env: process.env });
    child.on("error", (commandError) => rejectPromise(commandError));
    child.on("close", (code) => {
      if (code === 0) {
        resolvePromise();
        return;
      }
      rejectPromise(new Error(`${command} ${args.join(" ")} failed with exit code ${code ?? 1}`));
    });
  });
}

async function installCoreDependency(
  projectDir: string,
  packageManager: PackageManager,
  language: InitLanguage,
  provider: string
): Promise<void> {
  const providerPackage = getProviderPackageName(provider);
  const corePackages = providerPackage ? ["@runfabric/core", providerPackage] : ["@runfabric/core"];

  if (packageManager === "pnpm") {
    await runCommand("pnpm", ["add", ...corePackages], projectDir);
    if (language === "ts") {
      await runCommand("pnpm", ["add", "-D", "typescript", "@types/node"], projectDir);
    }
    return;
  }
  if (packageManager === "yarn") {
    await runCommand("yarn", ["add", ...corePackages], projectDir);
    if (language === "ts") {
      await runCommand("yarn", ["add", "-D", "typescript", "@types/node"], projectDir);
    }
    return;
  }
  if (packageManager === "bun") {
    await runCommand("bun", ["add", ...corePackages], projectDir);
    if (language === "ts") {
      await runCommand("bun", ["add", "-d", "typescript", "@types/node"], projectDir);
    }
    return;
  }

  await runCommand("npm", ["install", ...corePackages], projectDir);
  if (language === "ts") {
    await runCommand("npm", ["install", "-D", "typescript", "@types/node"], projectDir);
  }
}

function packageManagerRunArgs(packageManager: PackageManager, scriptName: string): [string, string[]] {
  if (packageManager === "yarn") {
    return ["yarn", [scriptName]];
  }
  return [packageManager, ["run", scriptName]];
}

async function installDependenciesIfNeeded(params: {
  projectDir: string;
  packageManager: PackageManager;
  language: InitLanguage;
  provider: string;
  skipInstall?: boolean;
}): Promise<void> {
  if (params.skipInstall) {
    info("dependency installation skipped");
    return;
  }

  info(`installing dependencies using ${params.packageManager}...`);
  try {
    await installCoreDependency(params.projectDir, params.packageManager, params.language, params.provider);
    const providerPackage = getProviderPackageName(params.provider);
    success(providerPackage ? `installed @runfabric/core and ${providerPackage}` : "installed @runfabric/core");
  } catch (installError) {
    const message = installError instanceof Error ? installError.message : String(installError);
    warn(`dependency installation failed: ${message}`);
    const providerPackage = getProviderPackageName(params.provider);
    const manualPackages = providerPackage ? ["@runfabric/core", providerPackage] : ["@runfabric/core"];
    warn(`run manually: (cd ${params.projectDir} && ${packageManagerAddCommand(params.packageManager, manualPackages)})`);
  }
}

async function runCallLocalIfRequested(params: {
  callLocal?: boolean;
  packageManager: PackageManager;
  projectDir: string;
}): Promise<void> {
  if (!params.callLocal) {
    return;
  }

  info("running local provider-mimic call...");
  const [command, args] = packageManagerRunArgs(params.packageManager, "call:local");
  try {
    await runCommand(command, args, params.projectDir);
    success("local call completed");
  } catch (callError) {
    const message = callError instanceof Error ? callError.message : String(callError);
    warn(`local call failed: ${message}`);
    warn(`run manually later: (cd ${params.projectDir} && ${command} ${args.join(" ")})`);
  }
}

function defaultServiceName(projectDir: string, fallback: string): string {
  const derived = normalizePackageName(basename(projectDir));
  return derived === "runfabric-service" ? fallback : derived;
}

function createStateNamespaceId(service: string): string {
  return `${normalizePackageName(service)}-${randomBytes(4).toString("hex")}`;
}

async function runInitCommand(options: InitCommandOptions): Promise<void> {
  const selections = await resolveSelections(options);
  if (!selections) {
    return;
  }

  const template = templateDefinitions[selections.templateName];
  const projectDir = resolve(options.dir);
  const service =
    options.service && options.service.trim().length > 0
      ? options.service.trim()
      : defaultServiceName(projectDir, template.defaultService);
  const stateNamespaceId = createStateNamespaceId(service);
  const credentialEnvVars = resolveCredentialEnvVars(projectDir, selections.provider);
  const paths = await writeProjectFiles({
    projectDir,
    service,
    stateNamespaceId,
    provider: selections.provider,
    language: selections.language,
    stateBackend: selections.stateBackend,
    packageManager: selections.packageManager,
    credentialEnvVars,
    skipInstall: Boolean(options.skipInstall),
    templateName: selections.templateName
  });

  logCreatedFiles(paths, selections.language);
  await installDependenciesIfNeeded({
    projectDir,
    packageManager: selections.packageManager,
    language: selections.language,
    provider: selections.provider,
    skipInstall: options.skipInstall
  });
  await runCallLocalIfRequested({
    callLocal: options.callLocal,
    packageManager: selections.packageManager,
    projectDir
  });

  success(`project scaffold initialized (${template.name}, ${selections.provider}, ${selections.language})`);
}

export const registerInitCommand: CommandRegistrar = (program) => {
  program
    .command("init")
    .description("Initialize a runfabric project scaffold")
    .option("--dir <path>", "Directory to initialize", ".")
    .option("--template <name>", "Template: api, worker, queue, cron")
    .option("--provider <name>", "Primary provider for generated config")
    .option("--state-backend <name>", "State backend: local, postgres, s3, gcs, azblob")
    .option("--lang <name>", "Source language: ts or js")
    .option("--pm <name>", "Package manager: npm, pnpm, yarn, bun")
    .option("--service <name>", "Service name override")
    .option("--skip-install", "Skip dependency installation")
    .option("--call-local", "Run local provider-mimic call after scaffold is generated")
    .option("--no-interactive", "Disable interactive prompts")
    .action(async (options: InitCommandOptions) => {
      try {
        await runInitCommand(options);
      } catch (initError) {
        const message = initError instanceof Error ? initError.message : String(initError);
        error(message);
        process.exitCode = 1;
      }
    });
};
