import type { CommandRegistrar } from "../types/cli";
import { randomBytes } from "node:crypto";
import { mkdir, writeFile } from "node:fs/promises";
import { basename, dirname, join, resolve } from "node:path";
import {
  normalizeRuntimeFamily,
  PROVIDER_IDS,
  runtimeFamilyList,
  type RuntimeFamily
} from "@runfabric/core";
import { createProviderRegistry } from "../providers/registry";
import { error, info, success } from "../utils/logger";
import { canPromptInteractively, promptSelection } from "./init/prompt";
import { installDependenciesIfNeeded, runCallLocalIfRequested } from "./init/dependencies";
import {
  languagePromptOptions,
  providerPromptOptions,
  runtimePromptOptions,
  stateBackendPromptOptions,
  templatePromptOptions
} from "./init/prompt-options";
import {
  isTemplateSupportedByProvider,
  supportedTemplatesForAnyProvider,
  supportedProvidersForTemplate,
  supportedTemplatesForProvider
} from "./init/template-support";
import {
  buildConfigContent,
  buildEnvExampleContent,
  buildGitIgnoreContent,
  buildHandlerContent,
  buildPackageJsonContent,
  buildProjectReadmeContent,
  buildTsConfigContent
} from "./init/render";
import {
  buildRunfabricSchemaContent,
  RUNFABRIC_SCHEMA_RELATIVE_PATH,
  RUNFABRIC_YAML_SCHEMA_DIRECTIVE
} from "./init/yaml-schema";
import {
  detectPackageManager,
  parsePackageManager
} from "./init/package-manager";
import {
  initLanguages,
  initStateBackends,
  isLanguage,
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
  runtime?: string;
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
  runtime: RuntimeFamily;
  language: InitLanguage;
  stateBackend: StateBackend;
  packageManager: PackageManager;
}

interface InitFilePaths {
  configPath: string;
  schemaPath: string;
  handlerPath: string;
  packageJsonPath: string;
  gitIgnorePath: string;
  envExamplePath: string;
  tsConfigPath: string;
  readmePath: string;
}
const SUPPORTED_TEMPLATE_NAMES = supportedTemplatesForAnyProvider();
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
function reportNoProvidersForTemplate(template: InitTemplateName): null {
  error(`no providers support template "${template}"`);
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
    return reportUnsupported(value, "template", [...SUPPORTED_TEMPLATE_NAMES]);
  }
  return validateTemplateSupport(value, provider);
}
async function resolveTemplateNameWithoutProvider(
  options: InitCommandOptions,
  interactiveMode: boolean
): Promise<InitTemplateName | null> {
  const defaultTemplate = SUPPORTED_TEMPLATE_NAMES.includes("api") ? "api" : SUPPORTED_TEMPLATE_NAMES[0];
  const value = options.template
    ? options.template
    : interactiveMode
      ? await promptSelection("Select template", templatePromptOptions(SUPPORTED_TEMPLATE_NAMES), defaultTemplate)
      : defaultTemplate;
  if (!isTemplateName(value)) {
    return reportUnsupported(value, "template", [...SUPPORTED_TEMPLATE_NAMES]);
  }
  return value;
}
async function resolveProviderName(
  options: InitCommandOptions,
  interactiveMode: boolean,
  allowedProviders: readonly (typeof PROVIDER_IDS)[number][] = PROVIDER_IDS,
  defaultProvider = "aws-lambda"
): Promise<string | null> {
  const candidates = [...allowedProviders];
  if (candidates.length === 0) {
    error("no providers available for selected init options");
    process.exitCode = 1;
    return null;
  }
  const defaultSelection = candidates.includes(defaultProvider as (typeof PROVIDER_IDS)[number])
    ? defaultProvider
    : candidates[0];
  const value = options.provider
    ? options.provider
    : interactiveMode
      ? await promptSelection("Select provider", providerPromptOptions(candidates), defaultSelection)
      : defaultSelection;
  if (!PROVIDER_IDS.includes(value as (typeof PROVIDER_IDS)[number])) {
    return reportUnsupported(value, "provider", [...PROVIDER_IDS]);
  }
  if (!candidates.includes(value as (typeof PROVIDER_IDS)[number])) {
    error(`provider "${value}" is not supported by selected template`);
    error(`supported providers: ${candidates.join(", ")}`);
    process.exitCode = 1;
    return null;
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

async function resolveRuntime(options: InitCommandOptions, interactiveMode: boolean): Promise<RuntimeFamily | null> {
  const value = options.runtime
    ? options.runtime
    : interactiveMode
      ? await promptSelection("Select runtime", runtimePromptOptions(), "nodejs")
      : "nodejs";
  const normalized = normalizeRuntimeFamily(value);
  if (!normalized) {
    return reportUnsupported(value, "runtime", runtimeFamilyList().split(" | "));
  }
  return normalized;
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
  const parsed = parsePackageManager(value);
  if (!parsed) {
    return reportUnsupported(value, "package manager", ["npm", "pnpm", "yarn", "bun"]);
  }
  return parsed;
}

async function resolveProviderAndTemplate(
  options: InitCommandOptions,
  interactiveMode: boolean
): Promise<{ provider: string; templateName: InitTemplateName } | null> {
  if (options.provider) {
    const provider = await resolveProviderName(options, interactiveMode);
    if (!provider) {
      return null;
    }
    const templateName = await resolveTemplateName(options, interactiveMode, provider);
    if (!templateName) {
      return null;
    }
    return { provider, templateName };
  }

  if (options.template || interactiveMode) {
    const templateName = await resolveTemplateNameWithoutProvider(options, interactiveMode);
    if (!templateName) {
      return null;
    }
    const supportedProviders = supportedProvidersForTemplate(templateName);
    if (supportedProviders.length === 0) {
      return reportNoProvidersForTemplate(templateName);
    }
    const defaultProvider = supportedProviders.includes("aws-lambda")
      ? "aws-lambda"
      : supportedProviders[0];
    const provider = await resolveProviderName(options, interactiveMode, supportedProviders, defaultProvider);
    if (!provider) {
      return null;
    }
    const validatedTemplate = validateTemplateSupport(templateName, provider);
    if (!validatedTemplate) {
      return null;
    }
    return { provider, templateName: validatedTemplate };
  }

  const provider = await resolveProviderName(options, interactiveMode);
  if (!provider) {
    return null;
  }
  const templateName = await resolveTemplateName(options, interactiveMode, provider);
  if (!templateName) {
    return null;
  }
  return { provider, templateName };
}

async function resolveSelections(options: InitCommandOptions): Promise<InitSelections | null> {
  const interactiveMode = options.interactive !== false && canPromptInteractively();
  const providerTemplate = await resolveProviderAndTemplate(options, interactiveMode);
  if (!providerTemplate) {
    return null;
  }
  const { provider, templateName } = providerTemplate;

  const language = await resolveLanguage(options, interactiveMode);
  if (!language) {
    return null;
  }
  const runtime = await resolveRuntime(options, interactiveMode);
  if (!runtime) {
    return null;
  }
  const stateBackend = await resolveStateBackend(options, interactiveMode);
  if (!stateBackend) {
    return null;
  }
  const packageManager = resolvePackageManager(options.pm || detectPackageManager(process.env.npm_config_user_agent));
  if (!packageManager) {
    return null;
  }
  return { templateName, provider, runtime, language, stateBackend, packageManager };
}

function initFilePaths(projectDir: string, extension: string): InitFilePaths {
  return {
    configPath: join(projectDir, "runfabric.yml"),
    schemaPath: join(projectDir, RUNFABRIC_SCHEMA_RELATIVE_PATH),
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
    runtime: RuntimeFamily;
    language: InitLanguage;
    stateBackend: StateBackend;
    stateNamespaceId: string;
  },
  template: (typeof templateDefinitions)[InitTemplateName]
): string {
  return `${RUNFABRIC_YAML_SCHEMA_DIRECTIVE}\n${buildConfigContent(
    template,
    params.service,
    params.provider,
    params.runtime,
    params.language,
    params.stateBackend,
    params.stateNamespaceId
  )}`;
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
  runtime: RuntimeFamily;
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

async function ensureEditorSchema(projectDir: string): Promise<string> {
  const schemaPath = join(projectDir, RUNFABRIC_SCHEMA_RELATIVE_PATH);
  await mkdir(dirname(schemaPath), { recursive: true });
  await writeFile(schemaPath, buildRunfabricSchemaContent(), "utf8");
  return schemaPath;
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
    runtime: selections.runtime,
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
  info(`created ${await ensureEditorSchema(projectDir)}`);
  await runCallLocalIfRequested({
    callLocal: options.callLocal,
    packageManager: selections.packageManager,
    projectDir
  });

  success(
    `project scaffold initialized (${template.name}, ${selections.provider}, runtime=${selections.runtime}, ${selections.language})`
  );
}

export const registerInitCommand: CommandRegistrar = (program) => {
  program
    .command("init")
    .description("Initialize a runfabric project scaffold")
    .option("--dir <path>", "Directory to initialize", ".")
    .option("--template <name>", `Template: ${SUPPORTED_TEMPLATE_NAMES.join(", ")}`)
    .option("--provider <name>", "Primary provider for generated config")
    .option("--runtime <name>", `Runtime family: ${runtimeFamilyList()}`)
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
