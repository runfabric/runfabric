import type { CommandRegistrar } from "../types/cli";
import { mkdir, writeFile } from "node:fs/promises";
import { join, resolve } from "node:path";
import { info, success } from "../utils/logger";

export const registerInitCommand: CommandRegistrar = (program) => {
  program
    .command("init")
    .description("Initialize a runfabric project scaffold")
    .option("--dir <path>", "Directory to initialize", ".")
    .action(async (options: { dir: string }) => {
      const projectDir = resolve(options.dir);
      await mkdir(join(projectDir, "src"), { recursive: true });

      const configPath = join(projectDir, "runfabric.yml");
      const handlerPath = join(projectDir, "src", "index.ts");

      await writeFile(
        configPath,
        [
          "service: hello-http",
          "runtime: nodejs",
          "entry: src/index.ts",
          "",
          "providers:",
          "  - aws-lambda",
          "",
          "triggers:",
          "  - type: http",
          "    method: GET",
          "    path: /hello",
          ""
        ].join("\n"),
        "utf8"
      );

      await writeFile(
        handlerPath,
        [
          'import type { UniversalHandler } from "@runfabric/core";',
          "",
          "export const handler: UniversalHandler = async () => ({",
          "  status: 200,",
          '  headers: { "content-type": "application/json" },',
          '  body: JSON.stringify({ message: "hello world" })',
          "});",
          ""
        ].join("\n"),
        "utf8"
      );

      info(`created ${configPath}`);
      info(`created ${handlerPath}`);
      success("project scaffold initialized");
    });
};
