import assert from "node:assert/strict";
import { mkdtemp, readFile, writeFile, chmod } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import { Client } from "@modelcontextprotocol/sdk/client/index.js";
import { InMemoryTransport } from "@modelcontextprotocol/sdk/inMemory.js";
import { CallToolResultSchema, ListToolsResultSchema } from "@modelcontextprotocol/sdk/types.js";
import { createServer } from "./index.js";

async function withClient<T>(fn: (client: Client) => Promise<T>): Promise<T> {
    const server = createServer();
    const client = new Client({ name: "runfabric-mcp-test", version: "0.0.0" }, { capabilities: {} });
    const [clientTransport, serverTransport] = InMemoryTransport.createLinkedPair();
    await Promise.all([server.connect(serverTransport), client.connect(clientTransport)]);
    try {
        return await fn(client);
    } finally {
        await Promise.all([clientTransport.close(), serverTransport.close()]);
    }
}

async function createFakeRunfabric(tmpDir: string, argsFile: string): Promise<string> {
    const fakePath = path.join(tmpDir, "fake-runfabric.sh");
    const script = [
        "#!/bin/sh",
        "printf '%s\\n' \"$@\" > \"$RUNFABRIC_TEST_ARGS_FILE\"",
        "echo '{\"ok\":true}'",
    ].join("\n");
    await writeFile(fakePath, script, "utf8");
    await chmod(fakePath, 0o755);
    process.env.RUNFABRIC_TEST_ARGS_FILE = argsFile;
    process.env.RUNFABRIC_CMD = fakePath;
    return fakePath;
}

test("listTools includes phase 9 MCP tool names", async () => {
    await withClient(async (client) => {
        const tools = await client.request({ method: "tools/list", params: {} }, ListToolsResultSchema);
        const names = new Set((tools.tools ?? []).map((t) => t.name));
        assert.equal(names.has("runfabric_generate"), true);
        assert.equal(names.has("runfabric_state"), true);
        assert.equal(names.has("runfabric_workflow"), true);
    });
});

test("phase 9 tools invoke expected runfabric subcommands", async () => {
    const tmpDir = await mkdtemp(path.join(os.tmpdir(), "runfabric-mcp-test-"));
    const argsFile = path.join(tmpDir, "argv.txt");
    await createFakeRunfabric(tmpDir, argsFile);

    await withClient(async (client) => {
        const calls = [
            {
                name: "runfabric_generate",
                args: { configPath: "runfabric.yml", stage: "dev", args: ["provider-override"] },
                expectPrefix: ["generate", "-c", "runfabric.yml", "--stage", "dev", "provider-override"],
            },
            {
                name: "runfabric_state",
                args: { configPath: "runfabric.yml", stage: "dev", command: "list", args: ["--all"] },
                expectPrefix: ["state", "-c", "runfabric.yml", "--stage", "dev", "list", "--all"],
            },
            {
                name: "runfabric_workflow",
                args: { configPath: "runfabric.yml", stage: "dev", command: "inspect", args: ["--run-id", "r1"] },
                expectPrefix: ["workflow", "-c", "runfabric.yml", "--stage", "dev", "inspect", "--run-id", "r1"],
            },
        ];

        for (const c of calls) {
            const result = await client.request(
                { method: "tools/call", params: { name: c.name, arguments: c.args } },
                CallToolResultSchema,
            );
            assert.equal(Boolean(result.isError), false, `${c.name} returned error: ${JSON.stringify(result.content)}`);
            const raw = await readFile(argsFile, "utf8");
            const argv = raw.split("\n").map((v) => v.trim()).filter((v) => v.length > 0);
            assert.deepEqual(argv.slice(0, c.expectPrefix.length), c.expectPrefix, `${c.name} argv prefix mismatch`);
            assert.deepEqual(argv.slice(-3), ["--json", "--non-interactive", "--yes"], `${c.name} missing MCP CLI flags`);
        }
    });
});