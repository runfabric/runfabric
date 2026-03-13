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

type DevPreset = "http" | "queue" | "storage" | "cron" | "eventbridge" | "pubsub" | "kafka" | "rabbitmq";

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
    if (
      normalized === "http" ||
      normalized === "queue" ||
      normalized === "storage" ||
      normalized === "cron" ||
      normalized === "eventbridge" ||
      normalized === "pubsub" ||
      normalized === "kafka" ||
      normalized === "rabbitmq"
    ) {
      return normalized;
    }
    throw new Error(
      `unsupported preset: ${value}. expected one of: http, queue, storage, cron, eventbridge, pubsub, kafka, rabbitmq`
    );
  }

  const firstType = project.triggers[0]?.type;
  if (firstType === TriggerEnum.Queue) {
    return "queue";
  }
  if (firstType === TriggerEnum.Storage) {
    return "storage";
  }
  if (firstType === TriggerEnum.Cron) {
    return "cron";
  }
  if (firstType === TriggerEnum.EventBridge) {
    return "eventbridge";
  }
  if (firstType === TriggerEnum.PubSub) {
    return "pubsub";
  }
  if (firstType === TriggerEnum.Kafka) {
    return "kafka";
  }
  if (firstType === TriggerEnum.RabbitMq) {
    return "rabbitmq";
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

function extractSchedule(project: ProjectConfig): string {
  const trigger = project.triggers.find((item) => item.type === TriggerEnum.Cron);
  return typeof trigger?.schedule === "string" && trigger.schedule.trim().length > 0
    ? trigger.schedule.trim()
    : "*/5 * * * *";
}

function extractPubSubTopic(project: ProjectConfig): string {
  const trigger = project.triggers.find((item) => item.type === TriggerEnum.PubSub);
  return typeof trigger?.topic === "string" && trigger.topic.trim().length > 0
    ? trigger.topic.trim()
    : "dev-topic";
}

function extractKafkaTarget(project: ProjectConfig): { topic: string; groupId: string } {
  const trigger = project.triggers.find((item) => item.type === TriggerEnum.Kafka);
  const topic =
    typeof trigger?.topic === "string" && trigger.topic.trim().length > 0 ? trigger.topic.trim() : "dev-topic";
  const groupId =
    typeof trigger?.groupId === "string" && trigger.groupId.trim().length > 0
      ? trigger.groupId.trim()
      : "dev-group";
  return { topic, groupId };
}

function extractRabbitMqTarget(project: ProjectConfig): { queue: string; routingKey: string } {
  const trigger = project.triggers.find((item) => item.type === TriggerEnum.RabbitMq);
  const queue =
    typeof trigger?.queue === "string" && trigger.queue.trim().length > 0 ? trigger.queue.trim() : "jobs";
  const routingKey =
    typeof trigger?.routingKey === "string" && trigger.routingKey.trim().length > 0
      ? trigger.routingKey.trim()
      : "jobs.created";
  return { queue, routingKey };
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

function createCronPresetEvent(provider: string, project: ProjectConfig): unknown {
  const schedule = extractSchedule(project);
  if (provider === "aws-lambda") {
    return {
      version: "0",
      id: `dev-cron-${Date.now()}`,
      "detail-type": "Scheduled Event",
      source: "aws.events",
      time: new Date().toISOString(),
      resources: [`arn:aws:events:us-east-1:000000000000:rule/${project.service}`],
      detail: { schedule }
    };
  }
  return {
    schedule,
    triggeredAt: new Date().toISOString()
  };
}

function createEventBridgePresetEvent(project: ProjectConfig): unknown {
  return {
    version: "0",
    id: `dev-eventbridge-${Date.now()}`,
    source: "runfabric.dev",
    "detail-type": "dev.event",
    time: new Date().toISOString(),
    detail: {
      service: project.service,
      message: "eventbridge preset simulation"
    }
  };
}

function createPubSubPresetEvent(provider: string, project: ProjectConfig): unknown {
  const topic = extractPubSubTopic(project);
  const message = { source: "runfabric-dev", topic };
  if (provider === "gcp-functions") {
    return {
      message: {
        data: Buffer.from(JSON.stringify(message)).toString("base64"),
        attributes: { topic }
      },
      subscription: `projects/dev/subscriptions/${topic}-sub`
    };
  }
  return { topic, message };
}

function createKafkaPresetEvent(project: ProjectConfig): unknown {
  const target = extractKafkaTarget(project);
  return {
    topic: target.topic,
    groupId: target.groupId,
    records: [
      {
        key: "dev-key",
        value: { source: "runfabric-dev", topic: target.topic, groupId: target.groupId },
        partition: 0,
        offset: 1
      }
    ]
  };
}

function createRabbitMqPresetEvent(project: ProjectConfig): unknown {
  const target = extractRabbitMqTarget(project);
  return {
    queue: target.queue,
    routingKey: target.routingKey,
    contentType: "application/json",
    body: {
      source: "runfabric-dev",
      queue: target.queue,
      routingKey: target.routingKey
    }
  };
}

function createEventPresetEvent(provider: string, project: ProjectConfig, preset: Exclude<DevPreset, "http">): unknown {
  if (preset === "queue") {
    return createQueuePresetEvent(provider, project);
  }
  if (preset === "storage") {
    return createStoragePresetEvent(provider, project);
  }
  if (preset === "cron") {
    return createCronPresetEvent(provider, project);
  }
  if (preset === "eventbridge") {
    return createEventBridgePresetEvent(project);
  }
  if (preset === "pubsub") {
    return createPubSubPresetEvent(provider, project);
  }
  if (preset === "kafka") {
    return createKafkaPresetEvent(project);
  }
  return createRabbitMqPresetEvent(project);
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

interface BuildWatcherRuntime {
  watchers: FSWatcher[];
  timer: NodeJS.Timeout | undefined;
  running: boolean;
  queued: boolean;
  active: boolean;
}

function createBuildWatcherRuntime(): BuildWatcherRuntime {
  return {
    watchers: [],
    timer: undefined,
    running: false,
    queued: false,
    active: true
  };
}

async function runWatcherBuild(
  runtime: BuildWatcherRuntime,
  projectDir: string,
  options: Pick<DevCommandOptions, "config" | "stage" | "out" | "watch">,
  reason: string,
  onBuilt?: () => Promise<void>
): Promise<void> {
  if (!runtime.active) {
    return;
  }
  if (runtime.running) {
    runtime.queued = true;
    return;
  }

  runtime.running = true;
  try {
    const ok = await runBuildCycle(projectDir, options, reason);
    if (ok && onBuilt) {
      await onBuilt();
    }
  } catch (buildError) {
    const message = buildError instanceof Error ? buildError.message : String(buildError);
    warn(`[dev] build failed (${reason}): ${message}`);
  } finally {
    runtime.running = false;
    if (runtime.queued) {
      runtime.queued = false;
      await runWatcherBuild(runtime, projectDir, options, "queued", onBuilt);
    }
  }
}

function scheduleWatcherBuild(
  runtime: BuildWatcherRuntime,
  projectDir: string,
  options: Pick<DevCommandOptions, "config" | "stage" | "out" | "watch">,
  reason: string,
  onBuilt?: () => Promise<void>
): void {
  if (runtime.timer) {
    clearTimeout(runtime.timer);
  }
  runtime.timer = setTimeout(() => {
    void runWatcherBuild(runtime, projectDir, options, reason, onBuilt);
  }, 250);
}

function registerBuildWatch(
  runtime: BuildWatcherRuntime,
  projectDir: string,
  options: Pick<DevCommandOptions, "config" | "stage" | "out" | "watch">,
  path: string,
  onBuilt?: () => Promise<void>,
  recursive = false
): void {
  if (!existsSync(path)) {
    return;
  }

  const onChange = (): void => {
    scheduleWatcherBuild(runtime, projectDir, options, `watch:${path}`, onBuilt);
  };

  try {
    runtime.watchers.push(watch(path, { recursive }, onChange));
  } catch {
    if (!recursive) {
      warn(`[dev] watcher unavailable for ${path}`);
      return;
    }
    try {
      runtime.watchers.push(watch(path, {}, onChange));
    } catch {
      warn(`[dev] watcher unavailable for ${path}`);
    }
  }
}

async function stopBuildWatcher(runtime: BuildWatcherRuntime): Promise<void> {
  runtime.active = false;
  if (runtime.timer) {
    clearTimeout(runtime.timer);
  }
  for (const watcher of runtime.watchers) {
    watcher.close();
  }
}

function startBuildWatcher(
  projectDir: string,
  options: Pick<DevCommandOptions, "config" | "stage" | "out" | "watch">,
  onBuilt?: () => Promise<void>
): { stop: () => Promise<void> } {
  const runtime = createBuildWatcherRuntime();
  void runWatcherBuild(runtime, projectDir, options, "initial", onBuilt);

  if (options.watch) {
    registerBuildWatch(runtime, projectDir, options, resolve(projectDir, "src"), onBuilt, true);
    registerBuildWatch(runtime, projectDir, options, resolve(projectDir, "runfabric.yml"), onBuilt);
    if (options.config) {
      registerBuildWatch(runtime, projectDir, options, resolve(projectDir, options.config), onBuilt);
    }
  }

  return {
    async stop(): Promise<void> {
      await stopBuildWatcher(runtime);
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
  const event = createEventPresetEvent(provider, context.project, preset);

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

async function readIncomingBody(request: AsyncIterable<string | Buffer>): Promise<string | undefined> {
  const bodyChunks: Buffer[] = [];
  for await (const chunk of request) {
    bodyChunks.push(typeof chunk === "string" ? Buffer.from(chunk) : chunk);
  }
  return bodyChunks.length > 0 ? Buffer.concat(bodyChunks).toString("utf8") : undefined;
}

function writeHttpDevError(response: { statusCode: number; setHeader: (name: string, value: string) => void; end: (payload: string) => void }, errorValue: unknown): void {
  const message = errorValue instanceof Error ? errorValue.message : String(errorValue);
  response.statusCode = 500;
  response.setHeader("content-type", "application/json");
  response.end(JSON.stringify({ error: message }));
}

async function forwardHttpRequestToLocalCall(
  projectDir: string,
  provider: string,
  options: DevCommandOptions,
  request: {
    method?: string;
    url?: string;
    headers: Record<string, string | string[] | undefined>;
    [Symbol.asyncIterator](): AsyncIterator<string | Buffer>;
  }
) {
  const requestBody = await readIncomingBody(request);
  const url = new URL(request.url || options.path || "/", `http://${options.host}:${options.port || 8787}`);

  return executeLocalCall(projectDir, {
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
}

async function listenHttpServer(
  server: ReturnType<typeof createServer>,
  host: string,
  port: number
): Promise<void> {
  await new Promise<void>((resolvePromise, rejectPromise) => {
    const onError = (listenError: Error & { code?: string }): void => {
      if (listenError.code === "EADDRINUSE") {
        rejectPromise(new Error(`port ${port} is already in use on ${host}. choose another port with --port.`));
        return;
      }
      rejectPromise(listenError);
    };

    server.once("error", onError);
    server.listen(port, host, () => {
      server.off("error", onError);
      resolvePromise();
    });
  });
}

async function runHttpDevServer(projectDir: string, options: DevCommandOptions): Promise<void> {
  const context = await loadPlanningContext(projectDir, options.config, options.stage);
  const provider = defaultProvider(context.project, options.provider);
  const server = createServer(async (request, response) => {
    try {
      const local = await forwardHttpRequestToLocalCall(projectDir, provider, options, request);

      response.statusCode = local.response.statusCode;
      for (const [key, value] of Object.entries(local.response.headers || {})) {
        response.setHeader(key, value);
      }
      response.end(local.response.body || "");
    } catch (serverError) {
      writeHttpDevError(response, serverError);
    }
  });

  await listenHttpServer(server, options.host, options.port);

  const address = server.address();
  const resolvedPort = address && typeof address === "object" ? address.port : options.port;
  info(`[dev] http preset server listening on http://${options.host}:${resolvedPort}`);
  info("[dev] press Ctrl+C to stop");

  await waitForShutdown(async () => {
    await new Promise<void>((resolvePromise) => server.close(() => resolvePromise()));
  });
}

async function runHttpPreset(projectDir: string, options: DevCommandOptions): Promise<void> {
  const watcher = startBuildWatcher(projectDir, options);
  try {
    await runHttpDevServer(projectDir, options);
  } finally {
    await watcher.stop();
  }
}

function startPresetInterval(
  projectDir: string,
  options: DevCommandOptions,
  preset: Exclude<DevPreset, "http">
): NodeJS.Timeout | undefined {
  if (options.intervalSeconds <= 0) {
    return undefined;
  }

  const interval = setInterval(() => {
    void simulateEventPreset(projectDir, options, preset).catch((simulateError) => {
      warn(`[dev] ${preset} interval simulation failed: ${String(simulateError)}`);
    });
  }, options.intervalSeconds * 1000);
  interval.unref();
  return interval;
}

async function runEventPreset(projectDir: string, options: DevCommandOptions, preset: Exclude<DevPreset, "http">): Promise<void> {
  const watcher = startBuildWatcher(projectDir, options, async () => simulateEventPreset(projectDir, options, preset));
  const interval = startPresetInterval(projectDir, options, preset);

  try {
    if (!options.watch || options.once) {
      return;
    }
    info(`[dev] ${preset} preset watch mode enabled. press Ctrl+C to stop`);
    await waitForShutdown(async () => undefined);
  } finally {
    if (interval) {
      clearInterval(interval);
    }
    await watcher.stop();
  }
}

async function runDevCommand(options: DevCommandOptions): Promise<void> {
  const projectDir = await resolveProjectDir(process.cwd(), options.config);
  const context = await loadPlanningContext(projectDir, options.config, options.stage);
  const preset = parsePreset(options.preset, context.project);
  if (preset === "http") {
    await runHttpPreset(projectDir, options);
    return;
  }
  await runEventPreset(projectDir, options, preset);
}

export const registerDevCommand: CommandRegistrar = (program) => {
  const command = program
    .command("dev")
    .description("Local dev loop with watch rebuild and event simulation presets")
    .option("-c, --config <path>", "Path to runfabric config")
    .option("-s, --stage <name>", "Stage name override")
    .option("-p, --provider <name>", "Provider to emulate")
    .option(
      "--preset <http|queue|storage|cron|eventbridge|pubsub|kafka|rabbitmq>",
      "Dev preset; defaults from first configured trigger"
    )
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
      "For event presets, auto-simulate on interval (0 disables)",
      parseIntervalSeconds,
      0
    );

  command.action(async (options: DevCommandOptions) => {
    try {
      await runDevCommand(options);
    } catch (devError) {
      const message = devError instanceof Error ? devError.message : String(devError);
      error(message);
      process.exitCode = 1;
    }
  });
};
