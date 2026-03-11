import { createServer } from "node:http";
import { existsSync, watch, type FSWatcher } from "node:fs";
import { mkdir, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { buildProject } from "@runfabric/builder";
import { TriggerEnum, type ProjectConfig } from "@runfabric/core";
import type { CommandRegistrar } from "../types/cli";
import { executeLocalCall } from "./call-local";
import { loadPlanningContext } from "../utils/load-config";
import { resolveProjectDir } from "../utils/resolve-project";
import { error, info, warn } from "../utils/logger";

type DevPreset = "http" | "queue" | "storage";

interface DevCommandOptions {
  config?: string;
  stage?: string;
  provider?: string;
  preset?: string;
  watch: boolean;
  once?: boolean;
  host: string;
  port: number;
  method: string;
  path: string;
  query?: string;
  body?: string;
  header: string[];
  out?: string;
  entry?: string;
  intervalSeconds: number;
}

function parsePort(value: string): number {
  const parsed = Number.parseInt(value, 10);
  if (!Number.isInteger(parsed) || parsed < 0 || parsed > 65535) {
    throw new Error(`invalid port: ${value}`);
  }
  return parsed;
}

function parseIntervalSeconds(value: string): number {
  const parsed = Number.parseInt(value, 10);
  if (!Number.isInteger(parsed) || parsed < 0) {
    throw new Error(`invalid interval seconds: ${value}`);
  }
  return parsed;
}

function collectHeader(value: string, previous: string[]): string[] {
  return [...previous, value];
}

function parsePreset(value: string | undefined, project: ProjectConfig): DevPreset {
  if (value) {
    const normalized = value.trim().toLowerCase();
    if (normalized === "http" || normalized === "queue" || normalized === "storage") {
      return normalized;
    }
    throw new Error(`unsupported preset: ${value}. expected one of: http, queue, storage`);
  }

  const firstType = project.triggers[0]?.type;
  if (firstType === TriggerEnum.Queue) {
    return "queue";
  }
  if (firstType === TriggerEnum.Storage) {
    return "storage";
  }
  return "http";
}

function defaultProvider(project: ProjectConfig, override?: string): string {
  return override || project.providers[0] || "aws-lambda";
}

function normalizeRequestHeaders(
  headers: Record<string, string | string[] | undefined>,
  extraHeaders: string[]
): string[] {
  const pairs: string[] = [];
  for (const [key, value] of Object.entries(headers)) {
    if (typeof value === "string") {
      pairs.push(`${key}:${value}`);
      continue;
    }
    if (Array.isArray(value)) {
      pairs.push(`${key}:${value.join(", ")}`);
    }
  }
  return [...pairs, ...extraHeaders];
}

function extractQueueName(project: ProjectConfig): string {
  const trigger = project.triggers.find((item) => item.type === TriggerEnum.Queue);
  return typeof trigger?.queue === "string" && trigger.queue.trim().length > 0
    ? trigger.queue.trim()
    : "dev-queue";
}

function extractStorageTarget(project: ProjectConfig): { bucket: string; eventName: string } {
  const trigger = project.triggers.find((item) => item.type === TriggerEnum.Storage);
  const bucket =
    typeof trigger?.bucket === "string" && trigger.bucket.trim().length > 0
      ? trigger.bucket.trim()
      : "dev-bucket";
  const eventName =
    Array.isArray(trigger?.events) && typeof trigger.events[0] === "string" && trigger.events[0].trim().length > 0
      ? trigger.events[0]
      : "ObjectCreated:Put";
  return { bucket, eventName };
}

function createQueuePresetEvent(provider: string, project: ProjectConfig): unknown {
  const queueName = extractQueueName(project);
  if (provider === "aws-lambda") {
    return {
      Records: [
        {
          messageId: "dev-message-1",
          receiptHandle: "dev-receipt-1",
          body: JSON.stringify({ source: "runfabric-dev", queue: queueName }),
          attributes: {
            ApproximateReceiveCount: "1"
          },
          messageAttributes: {},
          md5OfBody: "d41d8cd98f00b204e9800998ecf8427e",
          eventSource: "aws:sqs",
          eventSourceARN: `arn:aws:sqs:us-east-1:000000000000:${queueName}`,
          awsRegion: "us-east-1"
        }
      ]
    };
  }
  if (provider === "gcp-functions") {
    return {
      message: {
        data: Buffer.from(JSON.stringify({ source: "runfabric-dev", queue: queueName })).toString(
          "base64"
        ),
        attributes: {
          queue: queueName
        }
      },
      subscription: `projects/dev/subscriptions/${queueName}`
    };
  }
  return {
    queue: queueName,
    records: [{ body: { source: "runfabric-dev", queue: queueName } }]
  };
}

function createStoragePresetEvent(provider: string, project: ProjectConfig): unknown {
  const target = extractStorageTarget(project);
  if (provider === "aws-lambda") {
    return {
      Records: [
        {
          eventVersion: "2.1",
          eventSource: "aws:s3",
          awsRegion: "us-east-1",
          eventTime: new Date().toISOString(),
          eventName: target.eventName,
          s3: {
            bucket: { name: target.bucket },
            object: { key: "dev/object.json", size: 42 }
          }
        }
      ]
    };
  }
  if (provider === "gcp-functions") {
    return {
      bucket: target.bucket,
      name: "dev/object.json",
      contentType: "application/json",
      timeCreated: new Date().toISOString()
    };
  }
  return {
    bucket: target.bucket,
    key: "dev/object.json",
    event: target.eventName
  };
}

async function runBuildCycle(
  projectDir: string,
  options: Pick<DevCommandOptions, "config" | "stage" | "out">,
  reason: string
): Promise<boolean> {
  const context = await loadPlanningContext(projectDir, options.config, options.stage);
  if (!context.planning.ok) {
    warn(`[dev] build skipped (${reason}): planning has ${context.planning.errors.length} error(s)`);
    for (const planningError of context.planning.errors) {
      warn(`[dev] ${planningError}`);
    }
    return false;
  }

  await buildProject({
    planning: context.planning,
    project: context.project,
    projectDir,
    outputRoot: options.out
  });
  info(`[dev] build completed (${reason})`);
  return true;
}

function startBuildWatcher(
  projectDir: string,
  options: Pick<DevCommandOptions, "config" | "stage" | "out" | "watch">,
  onBuilt?: () => Promise<void>
): { stop: () => Promise<void> } {
  const watchers: FSWatcher[] = [];
  let timer: NodeJS.Timeout | undefined;
  let running = false;
  let queued = false;
  let active = true;

  const run = async (reason: string): Promise<void> => {
    if (!active) {
      return;
    }
    if (running) {
      queued = true;
      return;
    }
    running = true;
    try {
      const ok = await runBuildCycle(projectDir, options, reason);
      if (ok && onBuilt) {
        await onBuilt();
      }
    } catch (buildError) {
      const message = buildError instanceof Error ? buildError.message : String(buildError);
      warn(`[dev] build failed (${reason}): ${message}`);
    } finally {
      running = false;
      if (queued) {
        queued = false;
        await run("queued");
      }
    }
  };

  const schedule = (reason: string): void => {
    if (timer) {
      clearTimeout(timer);
    }
    timer = setTimeout(() => {
      void run(reason);
    }, 250);
  };

  const registerWatch = (path: string, recursive = false): void => {
    if (!existsSync(path)) {
      return;
    }

    try {
      const watcher = watch(path, { recursive }, () => schedule(`watch:${path}`));
      watchers.push(watcher);
    } catch {
      if (!recursive) {
        warn(`[dev] watcher unavailable for ${path}`);
        return;
      }
      try {
        const watcher = watch(path, {}, () => schedule(`watch:${path}`));
        watchers.push(watcher);
      } catch {
        warn(`[dev] watcher unavailable for ${path}`);
      }
    }
  };

  void run("initial");

  if (options.watch) {
    registerWatch(resolve(projectDir, "src"), true);
    registerWatch(resolve(projectDir, "runfabric.yml"));
    if (options.config) {
      registerWatch(resolve(projectDir, options.config));
    }
  }

  return {
    async stop(): Promise<void> {
      active = false;
      if (timer) {
        clearTimeout(timer);
      }
      for (const watcher of watchers) {
        watcher.close();
      }
    }
  };
}

async function waitForShutdown(onStop: () => Promise<void>): Promise<void> {
  await new Promise<void>((resolvePromise) => {
    let stopping = false;
    const shutdown = async (): Promise<void> => {
      if (stopping) {
        return;
      }
      stopping = true;
      process.off("SIGINT", shutdown);
      process.off("SIGTERM", shutdown);
      process.off("SIGQUIT", shutdown);
      await onStop();
      resolvePromise();
    };

    process.on("SIGINT", shutdown);
    process.on("SIGTERM", shutdown);
    process.on("SIGQUIT", shutdown);
  });
}

async function simulateEventPreset(
  projectDir: string,
  options: DevCommandOptions,
  preset: Exclude<DevPreset, "http">
): Promise<void> {
  const context = await loadPlanningContext(projectDir, options.config, options.stage);
  const provider = defaultProvider(context.project, options.provider);
  const event =
    preset === "queue"
      ? createQueuePresetEvent(provider, context.project)
      : createStoragePresetEvent(provider, context.project);

  const tempRoot = await mkdir(join(tmpdir(), "runfabric-dev"), { recursive: true }).then(() =>
    join(tmpdir(), "runfabric-dev")
  );
  const eventPath = join(tempRoot, `preset-${preset}-${Date.now()}.json`);
  await writeFile(eventPath, JSON.stringify(event, null, 2), "utf8");

  try {
    const response = await executeLocalCall(projectDir, {
      config: options.config,
      provider,
      method: "POST",
      path: options.path || "/dev/event",
      query: options.query || "",
      body: options.body,
      event: eventPath,
      header: options.header || [],
      entry: options.entry,
      serve: false,
      watch: options.watch,
      host: options.host,
      port: options.port
    });
    info(
      `[dev] ${preset} simulation provider=${provider} status=${response.response.statusCode}`
    );
  } finally {
    await rm(eventPath, { force: true });
  }
}

async function runHttpDevServer(projectDir: string, options: DevCommandOptions): Promise<void> {
  const context = await loadPlanningContext(projectDir, options.config, options.stage);
  const provider = defaultProvider(context.project, options.provider);
  const server = createServer(async (request, response) => {
    try {
      const bodyChunks: Buffer[] = [];
      for await (const chunk of request) {
        bodyChunks.push(typeof chunk === "string" ? Buffer.from(chunk) : chunk);
      }
      const requestBody = bodyChunks.length > 0 ? Buffer.concat(bodyChunks).toString("utf8") : undefined;
      const url = new URL(
        request.url || options.path || "/",
        `http://${options.host}:${options.port || 8787}`
      );

      const local = await executeLocalCall(projectDir, {
        config: options.config,
        provider,
        method: (request.method || options.method || "GET").toUpperCase(),
        path: url.pathname,
        query: url.search.startsWith("?") ? url.search.slice(1) : url.search,
        body: requestBody,
        header: normalizeRequestHeaders(request.headers, options.header || []),
        entry: options.entry,
        serve: false,
        watch: options.watch,
        host: options.host,
        port: options.port
      });

      response.statusCode = local.response.statusCode;
      for (const [key, value] of Object.entries(local.response.headers || {})) {
        response.setHeader(key, value);
      }
      response.end(local.response.body || "");
    } catch (serverError) {
      const message = serverError instanceof Error ? serverError.message : String(serverError);
      response.statusCode = 500;
      response.setHeader("content-type", "application/json");
      response.end(JSON.stringify({ error: message }));
    }
  });

  await new Promise<void>((resolvePromise, rejectPromise) => {
    const onError = (listenError: Error & { code?: string }): void => {
      if (listenError.code === "EADDRINUSE") {
        rejectPromise(
          new Error(
            `port ${options.port} is already in use on ${options.host}. choose another port with --port.`
          )
        );
        return;
      }
      rejectPromise(listenError);
    };

    server.once("error", onError);
    server.listen(options.port, options.host, () => {
      server.off("error", onError);
      resolvePromise();
    });
  });

  const address = server.address();
  const resolvedPort = address && typeof address === "object" ? address.port : options.port;
  info(`[dev] http preset server listening on http://${options.host}:${resolvedPort}`);
  info("[dev] press Ctrl+C to stop");

  await waitForShutdown(async () => {
    await new Promise<void>((resolvePromise) => server.close(() => resolvePromise()));
  });
}

export const registerDevCommand: CommandRegistrar = (program) => {
  program
    .command("dev")
    .description("Local dev loop with watch rebuild and event simulation presets")
    .option("-c, --config <path>", "Path to runfabric config")
    .option("-s, --stage <name>", "Stage name override")
    .option("-p, --provider <name>", "Provider to emulate")
    .option("--preset <http|queue|storage>", "Dev preset; defaults from first configured trigger")
    .option("--watch", "Watch files and rebuild/simulate on changes", true)
    .option("--no-watch", "Disable watch mode")
    .option("--once", "Run one simulation and exit (queue/storage presets)")
    .option("--host <host>", "Host for http preset", "127.0.0.1")
    .option("--port <number>", "Port for http preset", parsePort, 8787)
    .option("--method <method>", "HTTP method for http preset", "GET")
    .option("--path <path>", "HTTP path for http preset", "/hello")
    .option("--query <query>", "Query string for http preset", "")
    .option("--body <body>", "Body payload")
    .option("--header <key:value>", "Header pair (repeatable)", collectHeader, [])
    .option("--entry <path>", "Handler entry override")
    .option("-o, --out <path>", "Output directory for build artifacts")
    .option(
      "--interval-seconds <number>",
      "For queue/storage presets, auto-simulate on interval (0 disables)",
      parseIntervalSeconds,
      0
    )
    .action(async (options: DevCommandOptions) => {
      try {
        const projectDir = await resolveProjectDir(process.cwd(), options.config);
        const context = await loadPlanningContext(projectDir, options.config, options.stage);
        const preset = parsePreset(options.preset, context.project);

        if (preset === "http") {
          const watcher = startBuildWatcher(projectDir, options);
          try {
            await runHttpDevServer(projectDir, options);
          } finally {
            await watcher.stop();
          }
          return;
        }

        let interval: NodeJS.Timeout | undefined;
        const watcher = startBuildWatcher(projectDir, options, async () => {
          await simulateEventPreset(projectDir, options, preset);
        });

        try {
          if (options.intervalSeconds > 0) {
            interval = setInterval(() => {
              void simulateEventPreset(projectDir, options, preset).catch((simulateError) => {
                warn(`[dev] ${preset} interval simulation failed: ${String(simulateError)}`);
              });
            }, options.intervalSeconds * 1000);
            interval.unref();
          }

          if (!options.watch || options.once) {
            return;
          }

          info(`[dev] ${preset} preset watch mode enabled. press Ctrl+C to stop`);
          await waitForShutdown(async () => {
            return undefined;
          });
        } finally {
          if (interval) {
            clearInterval(interval);
          }
          await watcher.stop();
        }
      } catch (devError) {
        const message = devError instanceof Error ? devError.message : String(devError);
        error(message);
        process.exitCode = 1;
      }
    });
};
