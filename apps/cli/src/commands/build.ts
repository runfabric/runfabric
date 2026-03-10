import type { CommandRegistrar } from "../types/cli";
import { resolve } from "node:path";
import { buildProject } from "@runfabric/builder";
import { createPlan } from "@runfabric/planner";
import { loadPlanningContext } from "../utils/load-config";
import { loadLifecycleHooks } from "../utils/hooks";
import { resolveFunctionProject } from "../utils/project-functions";
import { printJson } from "../utils/output";
import { resolveProjectDir } from "../utils/resolve-project";
import { error, info } from "../utils/logger";

export const registerBuildCommand: CommandRegistrar = (program) => {
  program
    .command("build")
    .description("Build function artifacts for target providers")
    .option("-c, --config <path>", "Path to runfabric config")
    .option("-s, --stage <name>", "Stage name override")
    .option("-f, --function <name>", "Build a specific function")
    .option("-o, --out <path>", "Output directory")
    .option("--json", "Emit JSON output")
    .action(
      async (options: { config?: string; stage?: string; function?: string; out?: string; json?: boolean }) => {
      const configPath = options.config ? resolve(process.cwd(), options.config) : undefined;
      const projectDir = await resolveProjectDir(process.cwd(), options.config);
      const context = await loadPlanningContext(projectDir, configPath, options.stage);
      const project = resolveFunctionProject(context.project, options.function);
      const planning = project === context.project ? context.planning : createPlan(project);

      if (!planning.ok) {
        for (const planningError of planning.errors) {
          error(planningError);
        }
        process.exitCode = 1;
        return;
      }

      const hooks = await loadLifecycleHooks(project, projectDir);
      for (const hook of hooks) {
        await hook.beforeBuild?.({
          project,
          projectDir,
          outputRoot: options.out
        });
      }

      const result = await buildProject({
        planning,
        project,
        projectDir,
        outputRoot: options.out
      });

      for (const hook of hooks) {
        await hook.afterBuild?.({
          project,
          projectDir,
          outputRoot: options.out,
          artifacts: result.artifacts
        });
      }

      if (options.json) {
        printJson(result);
      } else {
        info(`built ${result.artifacts.length} artifact(s)`);
        for (const artifact of result.artifacts) {
          info(`${artifact.provider}: ${artifact.outputPath}`);
        }
      }
      }
    );
};
