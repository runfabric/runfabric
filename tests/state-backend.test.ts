import test from "node:test";
import assert from "node:assert/strict";
import { mkdtemp, mkdir, writeFile } from "node:fs/promises";
import { join } from "node:path";
import { tmpdir } from "node:os";
import {
  createStateBackend,
  LocalFileStateBackend,
  migrateStateBetweenBackends,
  normalizeStateConfig
} from "../packages/core/src/state.ts";
import { KeyValueStateBackend } from "../packages/core/src/state/keyvalue-backend.ts";

test("local state backend read/write/lock/unlock", async () => {
  const projectDir = await mkdtemp(join(tmpdir(), "runfabric-state-"));
  const backend = new LocalFileStateBackend({ projectDir });
  const address = {
    service: "state-test",
    stage: "default",
    provider: "cloudflare-workers"
  };

  const missing = await backend.read(address);
  assert.equal(missing, null);

  const lock = await backend.lock(address, "test-owner");
  await backend.write(
    address,
    {
      schemaVersion: 2,
      provider: address.provider,
      service: address.service,
      stage: address.stage,
      endpoint: "https://example.workers.dev",
      updatedAt: new Date().toISOString(),
      lifecycle: "applied",
      details: {
        deploymentId: "abc123"
      }
    },
    lock
  );
  await backend.unlock(address, lock);

  const stored = await backend.read(address);
  assert.ok(stored);
  assert.equal(stored?.provider, address.provider);
  assert.equal(stored?.stage, address.stage);
  assert.equal(stored?.endpoint, "https://example.workers.dev");
  assert.equal(stored?.lifecycle, "applied");
});

test("state backend fails fast on lock contention and supports force-unlock", async () => {
  const projectDir = await mkdtemp(join(tmpdir(), "runfabric-state-lock-"));
  const backend = new LocalFileStateBackend({ projectDir });
  const address = {
    service: "lock-test",
    stage: "default",
    provider: "aws-lambda"
  };

  const firstLock = await backend.lock(address, "owner-a");
  await assert.rejects(
    backend.lock(address, "owner-b"),
    /state lock is already held/,
    "second lock should fail with actionable lock contention error"
  );

  const removed = await backend.forceUnlock(address);
  assert.equal(removed, true);

  const secondLock = await backend.lock(address, "owner-b");
  assert.notEqual(firstLock.lockId, secondLock.lockId);
  await backend.unlock(address, secondLock);
});

test("local state backend rejects addresses that escape backend root", async () => {
  const projectDir = await mkdtemp(join(tmpdir(), "runfabric-state-path-"));
  const backend = new LocalFileStateBackend({ projectDir });
  const escapedAddress = {
    service: "..",
    stage: "..",
    provider: "aws-lambda"
  };

  await assert.rejects(
    backend.read(escapedAddress),
    /state address escapes configured backend root/,
    "read should reject path traversal-like address segments"
  );
  await assert.rejects(
    backend.lock(escapedAddress, "owner-a"),
    /state address escapes configured backend root/,
    "lock should reject path traversal-like address segments"
  );
});

test("local state backend surfaces parse failures during read/list", async () => {
  const projectDir = await mkdtemp(join(tmpdir(), "runfabric-state-parse-"));
  const backend = new LocalFileStateBackend({ projectDir });
  const address = {
    service: "parse-test",
    stage: "default",
    provider: "aws-lambda"
  };

  const stateFilePath = join(
    projectDir,
    ".runfabric",
    "state",
    address.service,
    address.stage,
    `${address.provider}.state.json`
  );
  await mkdir(join(projectDir, ".runfabric", "state", address.service, address.stage), { recursive: true });
  await writeFile(stateFilePath, "{ this-is-invalid-json", "utf8");

  await assert.rejects(
    backend.read(address),
    /failed to parse state record at .*aws-lambda\.state\.json/,
    "read should expose parse diagnostics with file path context"
  );
  await assert.rejects(
    backend.list(),
    /failed to parse state record at .*aws-lambda\.state\.json/,
    "list should fail loudly when a persisted state record is malformed"
  );
});

test("key-value state backend surfaces parse failures during read/list", async () => {
  const config = normalizeStateConfig({
    backend: "s3",
    s3: {
      bucket: "ignored-test-bucket",
      region: "us-east-1"
    }
  });
  const brokenStateKey = "runfabric/state/parse-test/default/aws-lambda.state.json";
  const backend = new KeyValueStateBackend({
    backend: "s3",
    config,
    keyPrefix: config.s3.keyPrefix,
    store: {
      async get(key: string): Promise<string | null> {
        return key === brokenStateKey ? "{ this-is-invalid-json" : null;
      },
      async put(): Promise<void> {},
      async delete(): Promise<void> {},
      async list(): Promise<string[]> {
        return [brokenStateKey];
      }
    }
  });

  await assert.rejects(
    backend.read({
      service: "parse-test",
      stage: "default",
      provider: "aws-lambda"
    }),
    /failed to parse state record at runfabric\/state\/parse-test\/default\/aws-lambda\.state\.json/,
    "read should expose parse diagnostics with object-key context"
  );
  await assert.rejects(
    backend.list(),
    /failed to parse state record at runfabric\/state\/parse-test\/default\/aws-lambda\.state\.json/,
    "list should fail loudly when a stored state record is malformed"
  );
});

test("state backend factory selects postgres and s3 backends", async () => {
  const projectDir = await mkdtemp(join(tmpdir(), "runfabric-state-backend-"));

  const postgres = createStateBackend({
    projectDir,
    state: {
      backend: "postgres",
      postgres: {
        schema: "public",
        table: "rf_state"
      }
    }
  });
  assert.equal(postgres.backend, "postgres");

  const s3 = createStateBackend({
    projectDir,
    state: {
      backend: "s3",
      s3: {
        bucket: "state-bucket",
        region: "us-east-1"
      }
    }
  });
  assert.equal(s3.backend, "s3");
});

test("state migration verifies checksum and record count", async () => {
  const projectDir = await mkdtemp(join(tmpdir(), "runfabric-state-migrate-"));

  const source = createStateBackend({
    projectDir,
    state: {
      backend: "local",
      local: {
        dir: "./.runfabric/state-source"
      }
    }
  });
  const target = createStateBackend({
    projectDir,
    state: {
      backend: "local",
      local: {
        dir: "./.runfabric/state-target"
      }
    }
  });

  const address = {
    service: "migrate-service",
    stage: "dev",
    provider: "aws-lambda"
  };

  const sourceLock = await source.lock(address, "migrate-source");
  await source.write(
    address,
    {
      schemaVersion: 2,
      provider: address.provider,
      service: address.service,
      stage: address.stage,
      lifecycle: "applied",
      updatedAt: new Date().toISOString(),
      endpoint: "https://example.execute-api.us-east-1.amazonaws.com/dev"
    },
    sourceLock
  );
  await source.unlock(address, sourceLock);

  const result = await migrateStateBetweenBackends(source, target, {
    service: address.service,
    stage: address.stage
  });
  assert.equal(result.copiedCount, 1);
  assert.equal(result.fromChecksum, result.toChecksum);

  const targetRecord = await target.read(address);
  assert.equal(targetRecord?.provider, "aws-lambda");
  assert.equal(targetRecord?.lifecycle, "applied");
});
