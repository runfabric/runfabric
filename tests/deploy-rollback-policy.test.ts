import test from "node:test";
import assert from "node:assert/strict";
import { mkdtemp } from "node:fs/promises";
import { join } from "node:path";
import { tmpdir } from "node:os";
import {
  TriggerEnum,
  normalizeStateConfig,
  type DeployFailure,
  type ProjectConfig,
  type ProviderAdapter,
  type ProviderCapabilities,
  type ProviderStateRecord,
  type StateAddress,
  type StateBackend,
  type StateLockInfo
} from "../packages/core/src/index.ts";
import { rollbackDeployments } from "../apps/cli/src/commands/deploy/workflow-provider.ts";
import type { DeployCollections, DeployContext, DeployWorkflowInput } from "../apps/cli/src/commands/deploy/workflow-types.ts";

function providerCapabilities(): ProviderCapabilities {
  return {
    http: true,
    cron: true,
    queue: true,
    storageEvent: true,
    eventbridge: true,
    pubsub: true,
    kafka: true,
    rabbitmq: true,
    streamingResponse: true,
    edgeRuntime: false,
    containerImage: true,
    customRuntime: true,
    backgroundJobs: true,
    websockets: true
  };
}

function createAdapter(
  name: string,
  overrides: Partial<ProviderAdapter> = {}
): ProviderAdapter {
  return {
    name,
    getCapabilities: () => providerCapabilities(),
    validate: async () => ({ ok: true, warnings: [], errors: [] }),
    planBuild: async () => ({ provider: name, steps: [] }),
    build: async () => ({ artifacts: [] }),
    planDeploy: async () => ({ provider: name, steps: [] }),
    deploy: async () => ({ provider: name }),
    ...overrides
  };
}

function createProject(deploy?: ProjectConfig["deploy"]): ProjectConfig {
  return {
    service: "rollback-test",
    runtime: "nodejs",
    entry: "src/index.ts",
    stage: "default",
    providers: ["aws-lambda"],
    triggers: [
      {
        type: TriggerEnum.Http,
        method: "GET",
        path: "/health"
      }
    ],
    deploy
  };
}

function createStateBackend(calls: {
  deletes: StateAddress[];
  forceUnlocks: StateAddress[];
}): StateBackend {
  const config = normalizeStateConfig({ backend: "local" });
  const lock: StateLockInfo = {
    backend: "local",
    lockId: "lock",
    owner: "owner",
    acquiredAt: new Date().toISOString(),
    expiresAt: new Date(Date.now() + 60_000).toISOString()
  };

  return {
    backend: "local",
    config,
    read: async (): Promise<ProviderStateRecord | null> => null,
    write: async (): Promise<void> => {},
    delete: async (address: StateAddress): Promise<void> => {
      calls.deletes.push(address);
    },
    list: async () => [],
    lock: async () => lock,
    renewLock: async () => lock,
    unlock: async (): Promise<void> => {},
    forceUnlock: async (address: StateAddress): Promise<boolean> => {
      calls.forceUnlocks.push(address);
      return true;
    },
    readLock: async (): Promise<StateLockInfo | null> => null,
    listLocks: async () => [],
    createBackup: async () => ({
      schemaVersion: 2,
      createdAt: new Date().toISOString(),
      backend: "local",
      records: [],
      locks: []
    }),
    restoreBackup: async (): Promise<void> => {}
  };
}

function createCollections(adapter: ProviderAdapter): DeployCollections {
  const failures: DeployFailure[] = [{ provider: "vercel", phase: "deploy", message: "simulated deploy failure" }];
  return {
    deployments: [{ provider: adapter.name, endpoint: "https://example.test" }],
    failures,
    successfulDeployments: [{ provider: adapter.name, adapter }],
    rollbacks: []
  };
}

function createContext(project: ProjectConfig, stateBackend: StateBackend): DeployContext {
  return {
    stage: "default",
    project,
    stateBackend,
    providerRegistry: {},
    hooks: [],
    buildResult: { artifacts: [] }
  };
}

test("rollbackDeployments honors CLI rollback flag", async () => {
  const projectDir = await mkdtemp(join(tmpdir(), "runfabric-rollback-cli-"));
  let destroyCalls = 0;
  const adapter = createAdapter("aws-lambda", {
    destroy: async () => {
      destroyCalls += 1;
    }
  });
  const backendCalls = { deletes: [] as StateAddress[], forceUnlocks: [] as StateAddress[] };
  const context = createContext(createProject(), createStateBackend(backendCalls));
  const collections = createCollections(adapter);
  const input: DeployWorkflowInput = {
    projectDir,
    rollbackOnFailure: true
  };

  await rollbackDeployments(input, context, collections);

  assert.equal(destroyCalls, 1);
  assert.equal(backendCalls.deletes.length, 1);
  assert.equal(backendCalls.forceUnlocks.length, 1);
  assert.equal(collections.deployments.length, 0);
  assert.deepEqual(collections.rollbacks, [
    {
      provider: "aws-lambda",
      ok: true,
      status: "succeeded"
    }
  ]);
});

test("rollbackDeployments honors project deploy.rollbackOnFailure when CLI flag is unset", async () => {
  const projectDir = await mkdtemp(join(tmpdir(), "runfabric-rollback-config-"));
  let destroyCalls = 0;
  const adapter = createAdapter("aws-lambda", {
    destroy: async () => {
      destroyCalls += 1;
    }
  });
  const backendCalls = { deletes: [] as StateAddress[], forceUnlocks: [] as StateAddress[] };
  const context = createContext(createProject({ rollbackOnFailure: true }), createStateBackend(backendCalls));
  const collections = createCollections(adapter);

  await rollbackDeployments({ projectDir }, context, collections);

  assert.equal(destroyCalls, 1);
  assert.equal(collections.rollbacks[0]?.status, "succeeded");
});

test("rollbackDeployments records explicit unsupported rollback when provider destroy is missing", async () => {
  const projectDir = await mkdtemp(join(tmpdir(), "runfabric-rollback-unsupported-"));
  const adapter = createAdapter("vercel");
  const backendCalls = { deletes: [] as StateAddress[], forceUnlocks: [] as StateAddress[] };
  const context = createContext(createProject({ rollbackOnFailure: true }), createStateBackend(backendCalls));
  const collections = createCollections(adapter);

  await rollbackDeployments({ projectDir }, context, collections);

  assert.equal(backendCalls.deletes.length, 0);
  assert.equal(backendCalls.forceUnlocks.length, 0);
  assert.deepEqual(collections.rollbacks, [
    {
      provider: "vercel",
      ok: false,
      status: "unsupported",
      message: "provider adapter does not implement destroy()"
    }
  ]);
  assert.ok(
    collections.failures.some(
      (failure) => failure.provider === "vercel" && failure.phase === "rollback" && failure.message.includes("unsupported")
    )
  );
});
