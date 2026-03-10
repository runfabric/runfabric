import type { CommandRegistrar } from "../types/cli";
import { mkdir, rm, writeFile } from "node:fs/promises";
import { resolve } from "node:path";
import { createProviderRegistry } from "../providers/registry";
import { loadPlanningContext } from "../utils/load-config";
import { printJson } from "../utils/output";
import { resolveProjectDir } from "../utils/resolve-project";
import { error, info, warn } from "../utils/logger";

interface RemoveFailure {
  provider: string;
  message: string;
}

interface RemoveRecoveryArtifact {
  provider: string;
  path: string;
}

export const registerRemoveCommand: CommandRegistrar = (program) => {
  program
    .command("remove")
    .description("Remove deployed artifacts/state and invoke provider cleanup flows")
    .option("-c, --config <path>", "Path to runfabric config")
    .option("-s, --stage <name>", "Stage name override")
    .option("-p, --provider <name>", "Limit removal to a single provider")
    .option("--json", "Emit JSON output")
    .action(
      async (options: { config?: string; stage?: string; provider?: string; json?: boolean }) => {
        const configPath = options.config ? resolve(process.cwd(), options.config) : undefined;
        const projectDir = await resolveProjectDir(process.cwd(), options.config);
        const context = await loadPlanningContext(projectDir, configPath, options.stage);
        const stage = context.project.stage || "default";
        const providers = options.provider ? [options.provider] : context.project.providers;
        const registry = createProviderRegistry(projectDir);
        const removedPaths: string[] = [];
        const destroyedProviders: string[] = [];
        const failures: RemoveFailure[] = [];
        const recoveryArtifacts: RemoveRecoveryArtifact[] = [];
        const recoveryDir = resolve(projectDir, ".runfabric", "recovery", "remove");
        await mkdir(recoveryDir, { recursive: true });

        for (const providerName of providers) {
          const provider = registry[providerName];
          if (!provider) {
            failures.push({
              provider: providerName,
              message: "provider adapter is not installed"
            });
            continue;
          }

          if (provider.destroy) {
            try {
              await provider.destroy(context.project);
              destroyedProviders.push(providerName);
            } catch (destroyError) {
              const message = `provider destroy failed: ${
                destroyError instanceof Error ? destroyError.message : String(destroyError)
              }`;
              failures.push({
                provider: providerName,
                message
              });

              const recoveryPath = resolve(
                recoveryDir,
                `${providerName}.${new Date().toISOString().replace(/[:.]/g, "-")}.json`
              );
              await writeFile(
                recoveryPath,
                JSON.stringify(
                  {
                    provider: providerName,
                    service: context.project.service,
                    stage,
                    message,
                    createdAt: new Date().toISOString(),
                    suggestion: "re-run runfabric remove after fixing provider credentials/permissions"
                  },
                  null,
                  2
                ),
                "utf8"
              );
              recoveryArtifacts.push({
                provider: providerName,
                path: recoveryPath
              });
              continue;
            }
          }

          const providerPaths = [
            resolve(projectDir, ".runfabric", "deploy", providerName),
            resolve(projectDir, ".runfabric", "build", providerName, context.project.service),
            resolve(
              projectDir,
              ".runfabric",
              "state",
              context.project.service,
              stage,
              `${providerName}.state.json`
            ),
            resolve(
              projectDir,
              ".runfabric",
              "state",
              context.project.service,
              stage,
              `${providerName}.state.json.lock`
            )
          ];

          for (const targetPath of providerPaths) {
            try {
              await rm(targetPath, { recursive: true, force: true });
              removedPaths.push(targetPath);
            } catch (removeError) {
              failures.push({
                provider: providerName,
                message: `failed to remove ${targetPath}: ${
                  removeError instanceof Error ? removeError.message : String(removeError)
                }`
              });
              const recoveryPath = resolve(
                recoveryDir,
                `${providerName}.${new Date().toISOString().replace(/[:.]/g, "-")}.json`
              );
              await writeFile(
                recoveryPath,
                JSON.stringify(
                  {
                    provider: providerName,
                    service: context.project.service,
                    stage,
                    failedPath: targetPath,
                    createdAt: new Date().toISOString(),
                    suggestion: "remove the path manually or re-run runfabric remove"
                  },
                  null,
                  2
                ),
                "utf8"
              );
              recoveryArtifacts.push({
                provider: providerName,
                path: recoveryPath
              });
            }
          }
        }

        const payload = {
          service: context.project.service,
          stage,
          providers,
          destroyedProviders,
          removedPaths,
          failures,
          recoveryArtifacts
        };

        if (failures.length > 0) {
          process.exitCode = 1;
        }

        if (options.json) {
          printJson(payload);
        } else {
          info(`remove completed for service ${context.project.service} (${stage})`);
          if (destroyedProviders.length > 0) {
            info(`provider destroy executed: ${destroyedProviders.join(", ")}`);
          }
          info(`removed paths: ${removedPaths.length}`);
          info(`recovery artifacts: ${recoveryArtifacts.length}`);
          if (failures.length > 0) {
            warn(`failures: ${failures.length}`);
            for (const failure of failures) {
              error(`${failure.provider}: ${failure.message}`);
            }
          }
        }
      }
    );
};
