import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";

const errors = [];

function addError(message) {
  errors.push(message);
}

function readText(path) {
  return readFileSync(resolve(path), "utf8");
}

function normalizeSet(values) {
  return [...new Set(values)].sort();
}

function sameSet(left, right) {
  const a = normalizeSet(left);
  const b = normalizeSet(right);
  return a.length === b.length && a.every((value, index) => value === b[index]);
}

function extractObjectLiteral(content, anchor) {
  const anchorIndex = content.search(anchor);
  if (anchorIndex < 0) {
    throw new Error(`Could not find anchor ${anchor}`);
  }

  const objectStart = content.indexOf("{", anchorIndex);
  if (objectStart < 0) {
    throw new Error("Could not find object start");
  }

  let depth = 0;
  let objectEnd = -1;
  for (let index = objectStart; index < content.length; index += 1) {
    const char = content[index];
    if (char === "{") {
      depth += 1;
    } else if (char === "}") {
      depth -= 1;
      if (depth === 0) {
        objectEnd = index;
        break;
      }
    }
  }

  if (objectEnd < 0) {
    throw new Error("Could not find object end");
  }

  return content.slice(objectStart, objectEnd + 1);
}

function parseTsObjectLiteral(objectLiteral) {
  const normalized = objectLiteral
    .replace(/\/\*[\s\S]*?\*\//g, "")
    .replace(/(^|\n)\s*\/\/.*$/g, "$1")
    .replace(/([{,]\s*)([A-Za-z_][A-Za-z0-9_]*)\s*:/g, '$1"$2":')
    .replace(/,\s*([}\]])/g, "$1");

  return JSON.parse(normalized);
}

function parseProviderIds() {
  const content = readText("packages/core/src/provider-ids.ts");
  const match = content.match(/export const PROVIDER_IDS\s*=\s*\[([\s\S]*?)\]\s*as const/);
  if (!match) {
    throw new Error("could not parse PROVIDER_IDS");
  }
  return normalizeSet([...match[1].matchAll(/"([^"]+)"/g)].map((entry) => entry[1]));
}

function extractSectionLines(markdown, heading) {
  const lines = markdown.split(/\r?\n/);
  const index = lines.findIndex((line) => line.trim() === heading);
  if (index < 0) {
    return [];
  }

  const out = [];
  for (let lineIndex = index + 1; lineIndex < lines.length; lineIndex += 1) {
    const line = lines[lineIndex];
    if (line.startsWith("## ")) {
      break;
    }
    out.push(line);
  }
  return out;
}

function getDocCommandLine(commandPrefix) {
  const content = readText("docs/site/command-reference.md");
  const lines = content.split(/\r?\n/);
  const line = lines.find((entry) => entry.includes(`\`${commandPrefix}`));
  if (!line) {
    addError(`command-reference missing line for "${commandPrefix}"`);
    return "";
  }
  return line;
}

function extractOptionSpecsFromSegment(segment) {
  const specs = [];
  for (const match of segment.matchAll(/\.(?:requiredOption|option)\("([^"]+)"/g)) {
    const signature = match[1];
    const longMatch = signature.match(/--([a-z0-9-]+)/i);
    const shortMatch = signature.match(/(^|,\s*)(-[a-zA-Z])(?=[\s,]|$)/);
    specs.push({
      long: longMatch ? `--${longMatch[1]}` : undefined,
      short: shortMatch ? shortMatch[2] : undefined
    });
  }
  return specs;
}

function extractCommandOptions(filePath, anchor) {
  const content = readText(filePath);
  const index = content.indexOf(anchor);
  if (index < 0) {
    addError(`${filePath}: missing anchor ${anchor}`);
    return [];
  }
  const tail = content.slice(index);
  const end = tail.indexOf(".action(");
  if (end < 0) {
    addError(`${filePath}: could not find .action() after ${anchor}`);
    return [];
  }
  return extractOptionSpecsFromSegment(tail.slice(0, end));
}

function validateCommandReference() {
  const checks = [
    {
      file: "apps/cli/src/commands/init.ts",
      anchor: '.command("init")',
      docCommand: "runfabric init"
    },
    {
      file: "apps/cli/src/commands/dev.ts",
      anchor: '.command("dev")',
      docCommand: "runfabric dev"
    },
    {
      file: "apps/cli/src/commands/call-local.ts",
      anchor: '.command("call-local")',
      docCommand: "runfabric call-local"
    },
    {
      file: "apps/cli/src/commands/invoke.ts",
      anchor: '.command("invoke")',
      docCommand: "runfabric invoke"
    },
    {
      file: "apps/cli/src/commands/deploy.ts",
      anchor: '.command("deploy")',
      docCommand: "runfabric deploy"
    },
    {
      file: "apps/cli/src/commands/migrate.ts",
      anchor: '.command("migrate")',
      docCommand: "runfabric migrate"
    },
    {
      file: "apps/cli/src/commands/compose.ts",
      anchor: '.command("plan")',
      docCommand: "runfabric compose plan"
    },
    {
      file: "apps/cli/src/commands/compose.ts",
      anchor: '.command("deploy")',
      docCommand: "runfabric compose deploy"
    },
    {
      file: "apps/cli/src/commands/state.ts",
      anchor: '.command("pull")',
      docCommand: "runfabric state pull"
    },
    {
      file: "apps/cli/src/commands/state.ts",
      anchor: '.command("list")',
      docCommand: "runfabric state list"
    },
    {
      file: "apps/cli/src/commands/state.ts",
      anchor: '.command("backup")',
      docCommand: "runfabric state backup"
    },
    {
      file: "apps/cli/src/commands/state.ts",
      anchor: '.command("restore")',
      docCommand: "runfabric state restore"
    },
    {
      file: "apps/cli/src/commands/state.ts",
      anchor: '.command("force-unlock")',
      docCommand: "runfabric state force-unlock"
    },
    {
      file: "apps/cli/src/commands/state.ts",
      anchor: '.command("migrate")',
      docCommand: "runfabric state migrate"
    },
    {
      file: "apps/cli/src/commands/state.ts",
      anchor: '.command("reconcile")',
      docCommand: "runfabric state reconcile"
    }
  ];

  for (const check of checks) {
    const expectedOptions = extractCommandOptions(check.file, check.anchor);
    const docLine = getDocCommandLine(check.docCommand);
    if (!docLine) {
      continue;
    }
    for (const option of expectedOptions) {
      const hasLong = option.long ? docLine.includes(option.long) : false;
      const hasShort = option.short ? docLine.includes(option.short) : false;
      if (!hasLong && !hasShort) {
        addError(
          `command-reference: "${check.docCommand}" missing ${option.long || option.short || "option"}`
        );
      }
    }
  }
}

function parseProviderCredentialSchemaEnvs() {
  const providerIds = parseProviderIds();
  const map = {};
  for (const providerId of providerIds) {
    const filePath = `packages/provider-${providerId}/src/provider.ts`;
    if (!existsSync(resolve(filePath))) {
      addError(`provider file missing: ${filePath}`);
      continue;
    }
    const content = readText(filePath);
    const schemaMatch = content.match(
      /const\s+\w+CredentialSchema[\s\S]*?fields:\s*\[([\s\S]*?)\]\s*};/
    );
    if (!schemaMatch) {
      addError(`${filePath}: could not parse credential schema fields`);
      continue;
    }
    map[providerId] = normalizeSet(
      [...schemaMatch[1].matchAll(/env:\s*"([A-Z0-9_]+)"/g)].map((entry) => entry[1])
    );
  }
  return map;
}

function parseTableRows(lines) {
  return lines
    .map((line) => line.trim())
    .filter((line) => line.startsWith("|"))
    .filter((line) => !line.includes("| ---"));
}

function parseCredentialsMatrixFromDocs() {
  const content = readText("docs/CREDENTIALS.md");
  const section = extractSectionLines(content, "## Provider Credential Matrix");
  const rows = parseTableRows(section).slice(1);

  const map = {};
  for (const row of rows) {
    const parts = row.split("|").map((entry) => entry.trim()).filter(Boolean);
    if (parts.length < 2) {
      continue;
    }
    const provider = (parts[0].match(/`([^`]+)`/) || [])[1];
    if (!provider) {
      continue;
    }
    const envs = normalizeSet([...parts[1].matchAll(/`([A-Z0-9_]+)`/g)].map((entry) => entry[1]));
    map[provider] = envs;
  }
  return map;
}

function parseRealDeployOverrideEnvsFromCode() {
  const providerIds = parseProviderIds();
  const map = {};
  for (const providerId of providerIds) {
    const filePath = `packages/provider-${providerId}/src/provider.ts`;
    if (!existsSync(resolve(filePath))) {
      continue;
    }
    const content = readText(filePath);
    map[providerId] = normalizeSet(
      [...content.matchAll(/RUNFABRIC_[A-Z0-9_]+_(?:DEPLOY|DESTROY)_CMD/g)].map(
        (entry) => entry[0]
      )
    );
  }
  return map;
}

function parseRealDeployOverrideEnvsFromDocs() {
  const content = readText("docs/CREDENTIALS.md");
  const section = extractSectionLines(content, "## Real Deploy Execution Matrix");
  const rows = parseTableRows(section).slice(1);

  const map = {};
  for (const row of rows) {
    const parts = row.split("|").map((entry) => entry.trim()).filter(Boolean);
    if (parts.length < 3) {
      continue;
    }
    const provider = (parts[0].match(/`([^`]+)`/) || [])[1];
    if (!provider) {
      continue;
    }
    map[provider] = normalizeSet(
      [...parts[2].matchAll(/`([A-Z0-9_]+_(?:DEPLOY|DESTROY)_CMD)`/g)].map(
        (entry) => entry[1]
      )
    );
  }
  return map;
}

function parseObservabilityMatrixFromDocs() {
  const content = readText("docs/CREDENTIALS.md");
  const section = extractSectionLines(content, "## Provider Observability Command Matrix");
  const rows = parseTableRows(section).slice(1);

  const map = {};
  for (const row of rows) {
    const parts = row.split("|").map((entry) => entry.trim()).filter(Boolean);
    if (parts.length < 3) {
      continue;
    }
    const provider = (parts[0].match(/`([^`]+)`/) || [])[1];
    if (!provider) {
      continue;
    }
    const tracesEnv = (parts[1].match(/`([A-Z0-9_]+)`/) || [])[1];
    const metricsEnv = (parts[2].match(/`([A-Z0-9_]+)`/) || [])[1];
    map[provider] = {
      tracesEnv,
      metricsEnv
    };
  }
  return map;
}

function parseObservabilityPrefixMapFromCode() {
  const content = readText("apps/cli/src/providers/registry.ts");
  const objectLiteral = extractObjectLiteral(content, /OBSERVABILITY_ENV_PREFIX\s*:\s*Record<.*?>\s*=\s*/);
  return parseTsObjectLiteral(objectLiteral);
}

function validateCredentialDocs() {
  const providerIds = parseProviderIds();

  const schemaEnvs = parseProviderCredentialSchemaEnvs();
  const docsCredentialMap = parseCredentialsMatrixFromDocs();
  const docCredentialProviders = normalizeSet(Object.keys(docsCredentialMap));
  if (!sameSet(providerIds, docCredentialProviders)) {
    addError(
      `credentials matrix providers mismatch. docs=${docCredentialProviders.join(",")} code=${providerIds.join(",")}`
    );
  }
  for (const providerId of providerIds) {
    const expected = schemaEnvs[providerId] || [];
    const actual = docsCredentialMap[providerId] || [];
    if (!sameSet(expected, actual)) {
      addError(
        `credentials matrix mismatch for ${providerId}. docs=${actual.join(",")} code=${expected.join(",")}`
      );
    }
  }

  const codeOverrideMap = parseRealDeployOverrideEnvsFromCode();
  const docsOverrideMap = parseRealDeployOverrideEnvsFromDocs();
  const docOverrideProviders = normalizeSet(Object.keys(docsOverrideMap));
  if (!sameSet(providerIds, docOverrideProviders)) {
    addError(
      `real deploy matrix providers mismatch. docs=${docOverrideProviders.join(",")} code=${providerIds.join(",")}`
    );
  }
  for (const providerId of providerIds) {
    const expected = codeOverrideMap[providerId] || [];
    const actual = docsOverrideMap[providerId] || [];
    if (!sameSet(expected, actual)) {
      addError(
        `real deploy override env mismatch for ${providerId}. docs=${actual.join(",")} code=${expected.join(",")}`
      );
    }
  }

  const observabilityPrefix = parseObservabilityPrefixMapFromCode();
  const docsObservability = parseObservabilityMatrixFromDocs();
  const docsObsProviders = normalizeSet(Object.keys(docsObservability));
  if (!sameSet(providerIds, docsObsProviders)) {
    addError(
      `observability matrix providers mismatch. docs=${docsObsProviders.join(",")} code=${providerIds.join(",")}`
    );
  }
  for (const providerId of providerIds) {
    const prefix = observabilityPrefix[providerId];
    const expected = {
      tracesEnv: prefix ? `RUNFABRIC_${prefix}_TRACES_CMD` : undefined,
      metricsEnv: prefix ? `RUNFABRIC_${prefix}_METRICS_CMD` : undefined
    };
    const actual = docsObservability[providerId];
    if (!actual || actual.tracesEnv !== expected.tracesEnv || actual.metricsEnv !== expected.metricsEnv) {
      addError(
        `observability env mismatch for ${providerId}. docs=${actual?.tracesEnv || "n/a"}/${actual?.metricsEnv || "n/a"} code=${expected.tracesEnv || "n/a"}/${expected.metricsEnv || "n/a"}`
      );
    }
  }
}

function parseCapabilityMatrixFromCode() {
  const content = readText("packages/planner/src/capability-matrix.ts");
  const objectLiteral = extractObjectLiteral(content, /capabilityMatrix\s*:\s*Record<.*?>\s*=\s*/);
  return parseTsObjectLiteral(objectLiteral);
}

function parseExamplesMatrixProviderConfigList() {
  const content = readText("docs/EXAMPLES_MATRIX.md");
  return normalizeSet(
    [...content.matchAll(/`runfabric\.([a-z0-9-]+)\.yml`/g)].map((entry) => entry[1])
  );
}

function parseTriggerCapabilityTableFromDocs() {
  const content = readText("docs/EXAMPLES_MATRIX.md");
  const section = extractSectionLines(content, "## Trigger Capability Matrix");
  const tableLines = parseTableRows(section);
  if (tableLines.length < 2) {
    addError("examples matrix trigger table is missing");
    return {};
  }
  const headerParts = tableLines[0]
    .split("|")
    .map((entry) => entry.trim())
    .filter(Boolean)
    .map((entry) => entry.toLowerCase());
  const rows = tableLines.slice(1);

  const out = {};
  for (const row of rows) {
    const parts = row.split("|").map((entry) => entry.trim()).filter(Boolean);
    if (parts.length !== headerParts.length) {
      continue;
    }
    const provider = parts[0];
    out[provider] = {};
    for (let index = 1; index < headerParts.length; index += 1) {
      out[provider][headerParts[index]] = parts[index];
    }
  }
  return out;
}

function validateExamplesMatrix() {
  const providerIds = parseProviderIds();
  const providerList = parseExamplesMatrixProviderConfigList();
  if (!sameSet(providerIds, providerList)) {
    addError(
      `examples matrix provider config list mismatch. docs=${providerList.join(",")} code=${providerIds.join(",")}`
    );
  }
  for (const providerId of providerIds) {
    const filePath = `examples/hello-http/runfabric.${providerId}.yml`;
    if (!existsSync(resolve(filePath))) {
      addError(`missing provider example config file: ${filePath}`);
    }
  }

  const codeMatrix = parseCapabilityMatrixFromCode();
  const docsMatrix = parseTriggerCapabilityTableFromDocs();
  const docProviders = normalizeSet(Object.keys(docsMatrix));
  if (!sameSet(providerIds, docProviders)) {
    addError(
      `examples trigger table providers mismatch. docs=${docProviders.join(",")} code=${providerIds.join(",")}`
    );
  }

  const columnMap = {
    http: "http",
    cron: "cron",
    queue: "queue",
    storage: "storageEvent",
    eventbridge: "eventbridge",
    pubsub: "pubsub",
    kafka: "kafka",
    rabbitmq: "rabbitmq"
  };

  for (const providerId of providerIds) {
    const docRow = docsMatrix[providerId] || {};
    const codeRow = codeMatrix[providerId];
    if (!codeRow) {
      addError(`capability matrix missing provider in code: ${providerId}`);
      continue;
    }
    for (const [docColumn, codeColumn] of Object.entries(columnMap)) {
      const expected = codeRow[codeColumn] ? "Y" : "N";
      const actual = docRow[docColumn];
      if (actual !== expected) {
        addError(
          `examples matrix mismatch ${providerId}.${docColumn}. docs=${actual || "n/a"} code=${expected}`
        );
      }
    }
  }
}

function main() {
  validateCommandReference();
  validateCredentialDocs();
  validateExamplesMatrix();

  if (errors.length > 0) {
    console.error("docs sync check failed:");
    for (const message of errors) {
      console.error(`- ${message}`);
    }
    process.exit(1);
  }

  console.log("docs sync is valid");
}

main();
