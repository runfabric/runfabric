import type { CommandRegistrar } from "../types/cli";
import { mkdir, writeFile } from "node:fs/promises";
import { spawn } from "node:child_process";
import { join, resolve } from "node:path";
import { stdin, stdout } from "node:process";
import { createInterface } from "node:readline/promises";
import { emitKeypressEvents } from "node:readline";
import { PROVIDER_IDS } from "@runfabric/core";
import { createProviderRegistry, getProviderPackageName } from "../providers/registry";
import { error, info, success, warn } from "../utils/logger";

type InitTemplateName = "api" | "worker" | "queue" | "cron";
type InitLanguage = "ts" | "js";
type PackageManager = "npm" | "pnpm" | "yarn" | "bun";
type StateBackend = "local" | "postgres" | "s3" | "gcs" | "azblob";

interface InitTemplateDefinition {
  name: InitTemplateName;
  defaultService: string;
  triggerBlock: string[];
  handlerBody: string[];
}

const templateDefinitions: Record<InitTemplateName, InitTemplateDefinition> = {
  api: {
    name: "api",
    defaultService: "hello-api",
    triggerBlock: [
      "triggers:",
      "  - type: http",
      "    method: GET",
      "    path: /hello"
    ],
    handlerBody: [
      '  body: JSON.stringify({ message: "hello from api template" })'
    ]
  },
  worker: {
    name: "worker",
    defaultService: "hello-worker",
    triggerBlock: [
      "triggers:",
      "  - type: http",
      "    method: POST",
      "    path: /work"
    ],
    handlerBody: [
      "  body: JSON.stringify({",
      '    message: "worker template accepted work",',
      "    received: req.body || null",
      "  })"
    ]
  },
  queue: {
    name: "queue",
    defaultService: "hello-queue",
    triggerBlock: [
      "triggers:",
      "  - type: queue",
      "    queue: jobs"
    ],
    handlerBody: [
      "  body: JSON.stringify({",
      '    message: "queue template processed message",',
      "    payload: req.body || null",
      "  })"
    ]
  },
  cron: {
    name: "cron",
    defaultService: "hello-cron",
    triggerBlock: [
      "triggers:",
      "  - type: cron",
      "    schedule: \"*/5 * * * *\""
    ],
    handlerBody: [
      "  body: JSON.stringify({",
      '    message: "cron template tick",',
      "    at: new Date().toISOString()",
      "  })"
    ]
  }
};

const initLanguages: InitLanguage[] = ["ts", "js"];
const initStateBackends: StateBackend[] = ["local", "postgres", "s3", "gcs", "azblob"];

function isTemplateName(value: string): value is InitTemplateName {
  return value === "api" || value === "worker" || value === "queue" || value === "cron";
}

function isLanguage(value: string): value is InitLanguage {
  return value === "ts" || value === "js";
}

function isPackageManager(value: string): value is PackageManager {
  return value === "npm" || value === "pnpm" || value === "yarn" || value === "bun";
}

function isStateBackend(value: string): value is StateBackend {
  return (
    value === "local" ||
    value === "postgres" ||
    value === "s3" ||
    value === "gcs" ||
    value === "azblob"
  );
}

function canPromptInteractively(): boolean {
  return Boolean(stdin.isTTY) && Boolean(stdout.isTTY);
}

function normalizePackageName(service: string): string {
  return service
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9-_]/g, "-")
    .replace(/--+/g, "-")
    .replace(/^-+/, "")
    .replace(/-+$/, "") || "runfabric-service";
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

async function promptSelection(
  question: string,
  choices: string[],
  defaultValue: string
): Promise<string> {
  if (!canPromptInteractively()) {
    return defaultValue;
  }

  const defaultIndex = Math.max(0, choices.indexOf(defaultValue));
  const input = stdin as NodeJS.ReadStream;
  const output = stdout;

  if (!input.isTTY || typeof input.setRawMode !== "function") {
    const rl = createInterface({
      input: stdin,
      output: stdout
    });

    info(question);
    choices.forEach((choice, index) => {
      info(`  ${index + 1}. ${choice}`);
    });

    const answer = (await rl.question(`Select [1-${choices.length}] (default ${defaultValue}): `)).trim();
    await rl.close();

    if (!answer) {
      return defaultValue;
    }
    const asIndex = Number(answer);
    if (Number.isInteger(asIndex) && asIndex >= 1 && asIndex <= choices.length) {
      return choices[asIndex - 1];
    }
    if (choices.includes(answer)) {
      return answer;
    }
    warn(`invalid selection "${answer}", using default "${defaultValue}"`);
    return defaultValue;
  }

  let selectedIndex = defaultIndex;
  const totalLines = choices.length + 2;

  const render = (firstRender = false): void => {
    if (!firstRender) {
      output.write(`\u001B[${totalLines}A`);
    }
    output.write("\u001B[0J");
    output.write(`${question}\n`);
    output.write("Use up/down arrows and Enter\n");
    for (let index = 0; index < choices.length; index += 1) {
      const prefix = index === selectedIndex ? ">" : " ";
      output.write(` ${prefix} ${choices[index]}\n`);
    }
  };

  return new Promise<string>((resolvePromise, rejectPromise) => {
    const cleanup = (): void => {
      input.off("keypress", onKeypress);
      input.setRawMode(false);
      input.pause();
      output.write("\u001B[?25h");
      output.write("\n");
    };

    const onKeypress = (_str: string, key: { name?: string; ctrl?: boolean } | undefined): void => {
      if (!key) {
        return;
      }

      if (key.ctrl && key.name === "c") {
        cleanup();
        rejectPromise(new Error("prompt cancelled by user"));
        return;
      }

      if (key.name === "up") {
        selectedIndex = (selectedIndex - 1 + choices.length) % choices.length;
        render();
        return;
      }

      if (key.name === "down") {
        selectedIndex = (selectedIndex + 1) % choices.length;
        render();
        return;
      }

      if (key.name === "return" || key.name === "enter") {
        const selected = choices[selectedIndex] || defaultValue;
        cleanup();
        resolvePromise(selected);
      }
    };

    emitKeypressEvents(input);
    input.setRawMode(true);
    input.resume();
    output.write("\u001B[?25l");
    render(true);
    input.on("keypress", onKeypress);
  });
}

function buildConfigContent(
  template: InitTemplateDefinition,
  service: string,
  provider: string,
  language: InitLanguage,
  stateBackend: StateBackend
): string {
  const extension = language === "ts" ? "ts" : "js";
  const stateLines = (() => {
    if (stateBackend === "local") {
      return [
        "state:",
        "  backend: local",
        "  local:",
        "    dir: ./.runfabric/state"
      ];
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
      return [
        "state:",
        "  backend: gcs",
        "  gcs:",
        "    bucket: ${env:RUNFABRIC_STATE_GCS_BUCKET}",
        "    prefix: runfabric/state"
      ];
    }
    return [
      "state:",
      "  backend: azblob",
      "  azblob:",
      "    container: ${env:RUNFABRIC_STATE_AZBLOB_CONTAINER}",
      "    prefix: runfabric/state"
    ];
  })();

  return [
    `service: ${service}`,
    "runtime: nodejs",
    `entry: src/index.${extension}`,
    "",
    "providers:",
    `  - ${provider}`,
    "",
    ...stateLines,
    "",
    ...template.triggerBlock,
    ""
  ].join("\n");
}

function buildHandlerContent(template: InitTemplateDefinition, language: InitLanguage): string {
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

function buildPackageJsonContent(
  service: string,
  language: InitLanguage,
  provider: string
): string {
  const scripts: Record<string, string> = {
    doctor: "runfabric doctor",
    plan: "runfabric plan",
    build: language === "ts" ? "tsc -p tsconfig.json" : "node -e \"console.log('no build step for js template')\"",
    deploy: "runfabric deploy",
    "call:local":
      language === "ts"
        ? "runfabric call-local -c runfabric.yml --serve --watch"
        : "runfabric call-local -c runfabric.yml --serve --watch"
  };

  if (language === "ts") {
    scripts.typecheck = "tsc --noEmit -p tsconfig.json";
  }

  const providerPackage = getProviderPackageName(provider);
  const dependencies: Record<string, string> = {
    "@runfabric/core": "^0.1.0"
  };
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
    packageJson.devDependencies = {
      typescript: "^5.9.2",
      "@types/node": "^24.5.2"
    };
  }

  return `${JSON.stringify(packageJson, null, 2)}\n`;
}

function buildTsConfigContent(): string {
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

function buildGitIgnoreContent(): string {
  return [
    "node_modules",
    "dist",
    ".runfabric",
    ".env",
    ""
  ].join("\n");
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

function buildProjectReadmeContent(params: {
  service: string;
  provider: string;
  language: InitLanguage;
  template: InitTemplateDefinition;
  stateBackend: StateBackend;
  packageManager: PackageManager;
  credentialEnvVars: string[];
  skippedInstall: boolean;
}): string {
  const commandPrefix = runCommandPrefix(params.packageManager);
  const extension = params.language === "ts" ? "ts" : "js";
  const credentialLines =
    params.credentialEnvVars.length > 0
      ? params.credentialEnvVars.map((envName) => `export ${envName}="your-value"`)
      : ['# no provider credential schema exposed for this provider'];
  const envFileLines =
    params.credentialEnvVars.length > 0
      ? params.credentialEnvVars.map((envName) => `${envName}=your-value`)
      : ["# credentials"];
  const stateBackendEnvHints = (() => {
    if (params.stateBackend === "local") {
      return ["# local backend selected; no additional state credentials required"];
    }
    if (params.stateBackend === "postgres") {
      return ['RUNFABRIC_STATE_POSTGRES_URL="postgres://user:pass@host:5432/dbname?sslmode=require"'];
    }
    if (params.stateBackend === "s3") {
      return [
        'RUNFABRIC_STATE_S3_BUCKET="your-state-bucket"',
        'AWS_REGION="us-east-1"',
        'AWS_ACCESS_KEY_ID="your-key"',
        'AWS_SECRET_ACCESS_KEY="your-secret"'
      ];
    }
    if (params.stateBackend === "gcs") {
      return [
        'RUNFABRIC_STATE_GCS_BUCKET="your-state-bucket"',
        'GOOGLE_APPLICATION_CREDENTIALS="/path/to/service-account.json"'
      ];
    }
    return [
      'RUNFABRIC_STATE_AZBLOB_CONTAINER="runfabric-state"',
      'AZURE_STORAGE_CONNECTION_STRING="your-connection-string"'
    ];
  })();

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
      : '// JavaScript template does not require type imports',
    "```",
    "",
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
    "## Local Call (Provider-mimic)",
    "",
    "Use built-in `runfabric call-local` to start a local HTTP server that forwards provider-shaped requests to your handler.",
    "",
    "```bash",
    `${commandPrefix} call:local`,
    `curl -i http://127.0.0.1:8787/hello`,
    "# stop server: Ctrl+C or type 'exit' and press Enter",
    `${commandPrefix} call:local -- --provider ${params.provider} --host 127.0.0.1 --port 8787 --serve --watch`,
    `${commandPrefix} call:local -- --provider ${params.provider} --method GET --path /hello`,
    `${commandPrefix} call:local -- --provider ${params.provider} --event ./event.json`,
    "```",
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
    params.language === "ts"
      ? "- TypeScript mode: if no built handler is found, `call-local` runs an initial `tsc -p tsconfig.json` automatically (requires `typescript` in dev dependencies)."
      : "- JavaScript mode: handler is loaded directly from configured entry.",
    "",
    "## Credentials",
    "",
    `Set credentials for \`${params.provider}\` in your shell before running deploy:`,
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
    "## State Backend",
    "",
    `Configured state backend in \`runfabric.yml\`: \`${params.stateBackend}\`.`,
    "",
    "Typical environment variables for this backend:",
    "",
    "```bash",
    ...stateBackendEnvHints,
    "```",
    "",
    "## Generated Files",
    "",
    "- `runfabric.yml`",
    `- \`src/index.${extension}\``,
    "- `package.json`",
    "- `.gitignore`",
    ...(params.language === "ts" ? ["- `tsconfig.json`"] : []),
    ...(params.skippedInstall ? ["", "Dependencies were not installed automatically (`--skip-install` was used)."] : []),
    ""
  ].join("\n");
}

async function runCommand(
  command: string,
  args: string[],
  cwd: string
): Promise<void> {
  await new Promise<void>((resolvePromise, rejectPromise) => {
    const child = spawn(command, args, {
      cwd,
      stdio: "inherit",
      env: process.env
    });

    child.on("error", (commandError) => {
      rejectPromise(commandError);
    });
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
  const corePackages = providerPackage
    ? ["@runfabric/core", providerPackage]
    : ["@runfabric/core"];

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
    .action(
      async (options: {
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
      }) => {
        const interactiveMode = options.interactive !== false && canPromptInteractively();

        const templateNameRaw = options.template
          ? options.template
          : interactiveMode
            ? await promptSelection("Select template", Object.keys(templateDefinitions), "api")
            : "api";
        if (!isTemplateName(templateNameRaw)) {
          error(`unknown template: ${templateNameRaw}`);
          error("supported templates: api, worker, queue, cron");
          process.exitCode = 1;
          return;
        }

        const providerRaw = options.provider
          ? options.provider
          : interactiveMode
            ? await promptSelection("Select provider", [...PROVIDER_IDS], "aws-lambda")
            : "aws-lambda";
        if (!PROVIDER_IDS.includes(providerRaw as (typeof PROVIDER_IDS)[number])) {
          error(`unknown provider: ${providerRaw}`);
          error(`supported providers: ${PROVIDER_IDS.join(", ")}`);
          process.exitCode = 1;
          return;
        }

        const languageRaw = options.lang
          ? options.lang
          : interactiveMode
            ? await promptSelection("Select language", [...initLanguages], "ts")
            : "ts";
        if (!isLanguage(languageRaw)) {
          error(`unknown language: ${languageRaw}`);
          error("supported languages: ts, js");
          process.exitCode = 1;
          return;
        }

        const stateBackendRaw = options.stateBackend
          ? options.stateBackend
          : interactiveMode
            ? await promptSelection("Select state backend", [...initStateBackends], "local")
            : "local";
        if (!isStateBackend(stateBackendRaw)) {
          error(`unknown state backend: ${stateBackendRaw}`);
          error(`supported state backends: ${initStateBackends.join(", ")}`);
          process.exitCode = 1;
          return;
        }

        const packageManagerRaw = options.pm || detectPackageManager();
        if (!isPackageManager(packageManagerRaw)) {
          error(`unknown package manager: ${packageManagerRaw}`);
          error("supported package managers: npm, pnpm, yarn, bun");
          process.exitCode = 1;
          return;
        }

        const template = templateDefinitions[templateNameRaw];
        const service = options.service && options.service.trim().length > 0
          ? options.service.trim()
          : template.defaultService;
        const language = languageRaw;
        const projectDir = resolve(options.dir);
        const extension = language === "ts" ? "ts" : "js";

        await mkdir(join(projectDir, "src"), { recursive: true });

        const configPath = join(projectDir, "runfabric.yml");
        const handlerPath = join(projectDir, "src", `index.${extension}`);
        const packageJsonPath = join(projectDir, "package.json");
        const gitIgnorePath = join(projectDir, ".gitignore");
        const tsConfigPath = join(projectDir, "tsconfig.json");
        const readmePath = join(projectDir, "README.md");

        const provider = createProviderRegistry(projectDir, [providerRaw])[providerRaw];
        const credentialSchema = provider?.getCredentialSchema?.();
        const credentialEnvVars =
          credentialSchema?.fields.map((field) => field.env).filter((value) => value.trim().length > 0) || [];

        await writeFile(
          configPath,
          buildConfigContent(template, service, providerRaw, language, stateBackendRaw),
          "utf8"
        );
        await writeFile(handlerPath, buildHandlerContent(template, language), "utf8");
        await writeFile(packageJsonPath, buildPackageJsonContent(service, language, providerRaw), "utf8");
        await writeFile(gitIgnorePath, buildGitIgnoreContent(), "utf8");
        await writeFile(
          readmePath,
          buildProjectReadmeContent({
            service,
            provider: providerRaw,
            language,
            template,
            stateBackend: stateBackendRaw,
            packageManager: packageManagerRaw,
            credentialEnvVars,
            skippedInstall: Boolean(options.skipInstall)
          }),
          "utf8"
        );
        if (language === "ts") {
          await writeFile(tsConfigPath, buildTsConfigContent(), "utf8");
        }

        info(`created ${configPath}`);
        info(`created ${handlerPath}`);
        info(`created ${packageJsonPath}`);
        info(`created ${gitIgnorePath}`);
        info(`created ${readmePath}`);
        if (language === "ts") {
          info(`created ${tsConfigPath}`);
        }

        if (!options.skipInstall) {
          info(`installing dependencies using ${packageManagerRaw}...`);
          try {
            await installCoreDependency(projectDir, packageManagerRaw, language, providerRaw);
            const providerPackage = getProviderPackageName(providerRaw);
            success(
              providerPackage
                ? `installed @runfabric/core and ${providerPackage}`
                : "installed @runfabric/core"
            );
          } catch (installError) {
            const message = installError instanceof Error ? installError.message : String(installError);
            warn(`dependency installation failed: ${message}`);
            const providerPackage = getProviderPackageName(providerRaw);
            const manualPackages = providerPackage
              ? ["@runfabric/core", providerPackage]
              : ["@runfabric/core"];
            warn(
              `run manually: (cd ${projectDir} && ${packageManagerAddCommand(packageManagerRaw, manualPackages)})`
            );
          }
        } else {
          info("dependency installation skipped");
        }

        if (options.callLocal) {
          info("running local provider-mimic call...");
          const [command, args] = packageManagerRunArgs(packageManagerRaw, "call:local");
          try {
            await runCommand(command, args, projectDir);
            success("local call completed");
          } catch (callError) {
            const message = callError instanceof Error ? callError.message : String(callError);
            warn(`local call failed: ${message}`);
            warn(`run manually later: (cd ${projectDir} && ${command} ${args.join(" ")})`);
          }
        }

        success(`project scaffold initialized (${template.name}, ${providerRaw}, ${language})`);
      }
    );
};
