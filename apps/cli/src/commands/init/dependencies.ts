import { spawn } from "node:child_process";
import { getProviderPackageName } from "../../providers/registry";
import { info, success, warn } from "../../utils/logger";
import { packageManagerRunArgs } from "./package-manager";
import { packageManagerAddCommand } from "./render";
import type { InitLanguage, PackageManager } from "./types";

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

export async function installDependenciesIfNeeded(params: {
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

export async function runCallLocalIfRequested(params: {
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
