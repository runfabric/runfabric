import test from "node:test";
import assert from "node:assert/strict";
import {
  MAX_CAPTURED_COMMAND_OUTPUT_BYTES,
  runJsonCommand,
  runShellCommand
} from "../packages/core/src/provider-ops/command-runner.ts";

test("runShellCommand caps stdout and stderr capture", async () => {
  const result = await runShellCommand(
    "node -e \"process.stdout.write('x'.repeat(400000)); process.stderr.write('y'.repeat(400000));\""
  );

  assert.equal(result.code, 0);
  assert.equal(result.stdoutTruncated, true);
  assert.equal(result.stderrTruncated, true);
  assert.match(result.stdout, /\[output truncated at \d+ bytes\]/);
  assert.match(result.stderr, /\[output truncated at \d+ bytes\]/);
  assert.ok(
    Buffer.byteLength(result.stdout, "utf8") <= MAX_CAPTURED_COMMAND_OUTPUT_BYTES + 128,
    "stdout should stay near configured capture limit"
  );
  assert.ok(
    Buffer.byteLength(result.stderr, "utf8") <= MAX_CAPTURED_COMMAND_OUTPUT_BYTES + 128,
    "stderr should stay near configured capture limit"
  );
});

test("runJsonCommand fails when stdout exceeds capture limit", async () => {
  await assert.rejects(
    runJsonCommand("node -e \"process.stdout.write(JSON.stringify({ value: 'x'.repeat(400000) }))\""),
    /command output exceeded/
  );
});
