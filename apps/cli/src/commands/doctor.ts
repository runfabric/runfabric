import type { CommandRegistrar } from "../types/cli";
import { resolve } from "node:path";
import { evaluateCredentialSchema } from "@runfabric/core";
import { createProviderRegistry, getProviderPackageName } from "../providers/registry";
import { loadPlanningContext } from "../utils/load-config";
import { printList } from "../utils/output";
import { resolveProjectDir } from "../utils/resolve-project";
import { error, info, success, warn } from "../utils/logger";

interface DoctorOptions {
  config?: string;
  stage?: string;
}

function checkNodeVersion(): boolean {
  const majorNodeVersion = Number(process.versions.node.split(".")[0]);
  if (majorNodeVersion < 20) {
    error(`node ${process.versions.node} is too old; require >= 20`);
    process.exitCode = 1;
    return false;
  }

  info(`node version ok: ${process.versions.node}`);
  return true;
}

function reportUnsupportedProviders(providers: string[]): void {
  const items = providers.map((provider) => {
    const packageName = getProviderPackageName(provider);
    return packageName ? `${provider} (install ${packageName})` : provider;
  });
  printList("Unsupported providers", items);
}

function reportPlanningIssues(warnings: string[], errorsList: string[]): boolean {
  for (const planningWarning of warnings) {
    warn(planningWarning);
  }

  if (errorsList.length === 0) {
    return true;
  }

  for (const planningError of errorsList) {
    error(planningError);
  }
  process.exitCode = 1;
  return false;
}

function reportCredentialIssues(
  providerNames: string[],
  registry: ReturnType<typeof createProviderRegistry>
): boolean {
  let hasCredentialErrors = false;

  for (const providerName of providerNames) {
    const provider = registry[providerName];
    if (!provider) {
      continue;
    }

    const credentialSchema = provider.getCredentialSchema?.();
    if (!credentialSchema) {
      warn(`${providerName}: credential schema is not defined; skipping credential check`);
      continue;
    }

    const credentialEvaluation = evaluateCredentialSchema(credentialSchema, process.env);
    for (const missing of credentialEvaluation.missingRequired) {
      error(`${providerName}: missing ${missing.env} (${missing.description})`);
      hasCredentialErrors = true;
    }
  }

  return !hasCredentialErrors;
}

async function executeDoctorCommand(options: DoctorOptions): Promise<void> {
  if (!checkNodeVersion()) {
    return;
  }

  const configPath = options.config ? resolve(process.cwd(), options.config) : undefined;
  const projectDir = await resolveProjectDir(process.cwd(), options.config);
  const context = await loadPlanningContext(projectDir, configPath, options.stage);
  const registry = createProviderRegistry(projectDir, context.project.providers);

  const unsupportedProviders = context.project.providers.filter((provider) => !registry[provider]);
  if (unsupportedProviders.length > 0) {
    reportUnsupportedProviders(unsupportedProviders);
    process.exitCode = 1;
    return;
  }

  if (!reportPlanningIssues(context.planning.warnings, context.planning.errors)) {
    return;
  }

  if (!reportCredentialIssues(context.project.providers, registry)) {
    process.exitCode = 1;
    return;
  }

  success("doctor checks passed");
}

export const registerDoctorCommand: CommandRegistrar = (program) => {
  program
    .command("doctor")
    .description("Run environment and project diagnostics")
    .option("-c, --config <path>", "Path to runfabric config")
    .option("-s, --stage <name>", "Stage name override")
    .action(async (options: DoctorOptions) => executeDoctorCommand(options));
};
