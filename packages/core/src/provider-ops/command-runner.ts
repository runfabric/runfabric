import { spawn } from "node:child_process";

export interface CommandResult {
  command: string;
  code: number;
  stdout: string;
  stderr: string;
  stdoutTruncated: boolean;
  stderrTruncated: boolean;
}

const RUNFABRIC_COMMAND_ENV_PATTERN = /^RUNFABRIC_[A-Z0-9_]+_CMD$/;
export const MAX_CAPTURED_COMMAND_OUTPUT_BYTES = 256 * 1024;
const FORBIDDEN_COMMAND_PATTERNS: Array<{ pattern: RegExp; label: string }> = [
  { pattern: /\r|\n|\0/, label: "newline/null byte" },
  { pattern: /&&|\|\|/, label: "command chaining (&&/||)" },
  { pattern: /(^|[^&])&([^&]|$)/, label: "background execution (&)" },
  { pattern: /[;|]/, label: "command separator (; or |)" },
  { pattern: /`|\$\(/, label: "shell execution (` or $())" },
  { pattern: /[<>]/, label: "shell redirection (< or >)" }
];

function appendOutputChunk(
  capture: { chunks: Buffer[]; bytes: number; truncated: boolean },
  chunk: Buffer | string
): void {
  if (capture.truncated) {
    return;
  }

  const buffer = Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk);
  const remaining = MAX_CAPTURED_COMMAND_OUTPUT_BYTES - capture.bytes;
  if (remaining <= 0) {
    capture.truncated = true;
    return;
  }

  if (buffer.byteLength <= remaining) {
    capture.chunks.push(buffer);
    capture.bytes += buffer.byteLength;
    return;
  }

  capture.chunks.push(buffer.subarray(0, remaining));
  capture.bytes = MAX_CAPTURED_COMMAND_OUTPUT_BYTES;
  capture.truncated = true;
}

function commandEnvName(command: string, env: Record<string, string | undefined> = process.env): string | undefined {
  for (const [name, value] of Object.entries(env)) {
    if (!RUNFABRIC_COMMAND_ENV_PATTERN.test(name)) {
      continue;
    }
    if (typeof value === "string" && value === command) {
      return name;
    }
  }
  return undefined;
}

function assertSafeRunfabricCommand(command: string, envName: string): void {
  const trimmed = command.trim();
  if (trimmed.length === 0) {
    throw new Error(`${envName} is empty; provide a non-empty command.`);
  }
  if (trimmed.length > 4000) {
    throw new Error(`${envName} is too long; keep command length under 4000 characters.`);
  }
  for (const rule of FORBIDDEN_COMMAND_PATTERNS) {
    if (!rule.pattern.test(command)) {
      continue;
    }
    throw new Error(
      `unsafe command in ${envName}: ${rule.label} is not allowed. Use a single executable command (for example: node ./scripts/deploy.cjs).`
    );
  }
}

export async function runShellCommand(
  command: string,
  options?: { cwd?: string; env?: Record<string, string | undefined> }
): Promise<CommandResult> {
  const envName = commandEnvName(command);
  if (envName) {
    assertSafeRunfabricCommand(command, envName);
  }

  return new Promise((resolvePromise, rejectPromise) => {
    const child = spawn("sh", ["-lc", command], {
      cwd: options?.cwd,
      env: {
        ...process.env,
        ...(options?.env || {})
      },
      stdio: ["ignore", "pipe", "pipe"]
    });

    const stdoutCapture = { chunks: [] as Buffer[], bytes: 0, truncated: false };
    const stderrCapture = { chunks: [] as Buffer[], bytes: 0, truncated: false };

    child.stdout.on("data", (chunk: Buffer | string) => {
      appendOutputChunk(stdoutCapture, chunk);
    });
    child.stderr.on("data", (chunk: Buffer | string) => {
      appendOutputChunk(stderrCapture, chunk);
    });
    child.on("error", (error) => {
      rejectPromise(error);
    });
    child.on("close", (code) => {
      const stdoutOutput = Buffer.concat(stdoutCapture.chunks, stdoutCapture.bytes).toString("utf8");
      const stderrOutput = Buffer.concat(stderrCapture.chunks, stderrCapture.bytes).toString("utf8");
      resolvePromise({
        command,
        code: code ?? 1,
        stdout: stdoutCapture.truncated
          ? `${stdoutOutput}\n[output truncated at ${MAX_CAPTURED_COMMAND_OUTPUT_BYTES} bytes]`
          : stdoutOutput,
        stderr: stderrCapture.truncated
          ? `${stderrOutput}\n[output truncated at ${MAX_CAPTURED_COMMAND_OUTPUT_BYTES} bytes]`
          : stderrOutput,
        stdoutTruncated: stdoutCapture.truncated,
        stderrTruncated: stderrCapture.truncated
      });
    });
  });
}

export async function runJsonCommand(
  command: string,
  options?: { cwd?: string; env?: Record<string, string | undefined> }
): Promise<unknown> {
  const result = await runShellCommand(command, options);
  if (result.code !== 0) {
    throw new Error(
      `command failed (${result.code}): ${command}\n${result.stderr || result.stdout || "no output"}`
    );
  }
  if (result.stdoutTruncated) {
    throw new Error(
      `command output exceeded ${MAX_CAPTURED_COMMAND_OUTPUT_BYTES} bytes; keep JSON output within limit: ${command}`
    );
  }

  const output = result.stdout.trim();
  if (!output) {
    throw new Error(`command produced empty output: ${command}`);
  }

  try {
    return JSON.parse(output) as unknown;
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    throw new Error(`command output is not valid JSON: ${message}`);
  }
}
