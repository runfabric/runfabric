import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative } from "node:path";
import { spawnSync } from "node:child_process";
import { createRequire } from "node:module";

const requireModule = createRequire(import.meta.url);
const ts = requireModule("typescript");

const syntaxRoots = ["apps", "packages", "tests"];
const guardrailRoots = ["apps", "packages"];
const syntaxFiles = [];
const guardrailFiles = [];
const ignoredDirectories = new Set(["node_modules", "dist", ".git"]);

const MAX_LINES_PER_FUNCTION = 50;
const MAX_LINES_PER_FILE = 600;
const MAX_CYCLOMATIC_COMPLEXITY = 20;
const MAX_TOP_LEVEL_FUNCTIONS_PER_FILE = 5;
const MAX_CLASSES_PER_FILE = 1;
const MAX_TYPE_DECLARATIONS_PER_FILE = 1;

// Progressive rollout baselines for the new structure guardrails.
const LEGACY_MAX_TOP_LEVEL_FUNCTIONS_PER_FILE_EXCEPTIONS = new Set([
  "apps/cli/src/commands/call-local/runtime.ts",
  "apps/cli/src/commands/call-local/serve.ts",
  "apps/cli/src/commands/compose.ts",
  "apps/cli/src/commands/deploy/workflow-provider.ts",
  "apps/cli/src/commands/deploy/workflow.ts",
  "apps/cli/src/commands/dev.ts",
  "apps/cli/src/commands/docs.ts",
  "apps/cli/src/commands/docs/render.ts",
  "apps/cli/src/commands/doctor.ts",
  "apps/cli/src/commands/init.ts",
  "apps/cli/src/commands/init/render.ts",
  "apps/cli/src/commands/migrate/parser.ts",
  "apps/cli/src/commands/remove.ts",
  "apps/cli/src/commands/state.ts",
  "apps/cli/src/providers/registry.ts",
  "packages/builder/src/index.ts",
  "packages/core/src/provider-ops.ts",
  "packages/core/src/provider-ops/payload.ts",
  "packages/core/src/state/backup.ts",
  "packages/core/src/state/config-utils.ts",
  "packages/core/src/state/key-value-stores.ts",
  "packages/core/src/state/record-utils.ts",
  "packages/planner/src/parse-config.ts",
  "packages/planner/src/parse-config/extensions.ts",
  "packages/planner/src/parse-config/project-readers.ts",
  "packages/planner/src/parse-config/shared.ts",
  "packages/planner/src/parse-config/state-config.ts",
  "packages/planner/src/parse-config/triggers.ts",
  "packages/planner/src/parse-config/yaml.ts",
  "packages/planner/src/planner.ts",
  "packages/provider-alibaba-fc/src/provider.ts",
  "packages/provider-aws-lambda/src/deploy-internal.ts",
  "packages/provider-aws-lambda/src/provider-metadata.ts",
  "packages/provider-aws-lambda/src/provider.ts",
  "packages/provider-azure-functions/src/provider.ts",
  "packages/provider-cloudflare-workers/src/provider.ts",
  "packages/provider-digitalocean-functions/src/provider.ts",
  "packages/provider-fly-machines/src/provider.ts",
  "packages/provider-gcp-functions/src/provider.ts",
  "packages/provider-ibm-openwhisk/src/provider.ts",
  "packages/provider-netlify/src/provider.ts",
  "packages/provider-vercel/src/provider.ts",
  "packages/runtime-node/src/framework-wrappers.ts"
]);

const LEGACY_MAX_CLASSES_PER_FILE_EXCEPTIONS = new Set([
  "packages/core/src/state/file-backend.ts",
  "packages/core/src/state/key-value-stores.ts",
  "packages/core/src/state/keyvalue-backend.ts"
]);

const LEGACY_MAX_TYPE_DECLARATIONS_PER_FILE_EXCEPTIONS = new Set([
  "apps/cli/src/commands/call-local/runtime.ts",
  "apps/cli/src/commands/deploy/workflow-types.ts",
  "apps/cli/src/commands/dev.ts",
  "apps/cli/src/commands/docs.ts",
  "apps/cli/src/commands/init.ts",
  "apps/cli/src/commands/init/types.ts",
  "apps/cli/src/commands/migrate/parser.ts",
  "apps/cli/src/commands/remove.ts",
  "apps/cli/src/commands/state.ts",
  "apps/cli/src/providers/registry.ts",
  "apps/cli/src/utils/compose.ts",
  "packages/builder/src/index.ts",
  "packages/core/src/credentials.ts",
  "packages/core/src/enums.ts",
  "packages/core/src/hooks.ts",
  "packages/core/src/primitives.ts",
  "packages/core/src/project.ts",
  "packages/core/src/provider-ops.ts",
  "packages/core/src/provider.ts",
  "packages/core/src/state.ts",
  "packages/core/src/state/key-value-stores.ts",
  "packages/core/src/types.ts",
  "packages/planner/src/parse-config/state-config.ts",
  "packages/planner/src/parse-config/yaml.ts",
  "packages/planner/src/planner.ts",
  "packages/provider-aws-lambda/src/provider.ts",
  "packages/provider-cloudflare-workers/src/provider.ts",
  "packages/runtime-node/src/framework-wrappers.ts"
]);

const COMMON_FUNCTION_USAGE_THRESHOLD = 3;

function toPosixPath(path) {
  return path.split("\\").join("/");
}

function toRepoRelativePath(path) {
  return toPosixPath(relative(process.cwd(), path));
}

function walk(directory, out) {
  for (const entry of readdirSync(directory)) {
    if (ignoredDirectories.has(entry)) {
      continue;
    }
    const target = join(directory, entry);
    const stat = statSync(target);
    if (stat.isDirectory()) {
      walk(target, out);
      continue;
    }
    if (target.endsWith(".ts") && !target.endsWith(".d.ts")) {
      out.push(target);
    }
  }
}

for (const root of syntaxRoots) {
  walk(root, syntaxFiles);
}
for (const root of guardrailRoots) {
  walk(root, guardrailFiles);
}

for (const file of syntaxFiles) {
  const result = spawnSync("node", ["--check", file], { stdio: "inherit" });
  if (result.status !== 0) {
    process.exit(result.status ?? 1);
  }
}

function isTopLevelNode(node) {
  return Boolean(node.parent && ts.isSourceFile(node.parent));
}

function functionLikeName(node) {
  if (node.name && ts.isIdentifier(node.name)) {
    return node.name.text;
  }

  if (ts.isConstructorDeclaration(node)) {
    return "constructor";
  }

  if (node.parent && ts.isVariableDeclaration(node.parent) && ts.isIdentifier(node.parent.name)) {
    return node.parent.name.text;
  }

  if (node.parent && ts.isPropertyAssignment(node.parent) && ts.isIdentifier(node.parent.name)) {
    return node.parent.name.text;
  }

  if (node.parent && ts.isMethodDeclaration(node.parent) && ts.isIdentifier(node.parent.name)) {
    return node.parent.name.text;
  }

  return "<anonymous>";
}

function computeCyclomaticComplexity(functionNode) {
  let complexity = 1;

  const visit = (node) => {
    if (node !== functionNode && ts.isFunctionLike(node)) {
      return;
    }

    if (
      ts.isIfStatement(node) ||
      ts.isForStatement(node) ||
      ts.isForInStatement(node) ||
      ts.isForOfStatement(node) ||
      ts.isWhileStatement(node) ||
      ts.isDoStatement(node) ||
      ts.isCatchClause(node) ||
      ts.isConditionalExpression(node)
    ) {
      complexity += 1;
    }

    if (ts.isCaseClause(node)) {
      complexity += 1;
    }

    if (ts.isBinaryExpression(node)) {
      const operator = node.operatorToken.kind;
      if (
        operator === ts.SyntaxKind.AmpersandAmpersandToken ||
        operator === ts.SyntaxKind.BarBarToken ||
        operator === ts.SyntaxKind.QuestionQuestionToken
      ) {
        complexity += 1;
      }
    }

    ts.forEachChild(node, visit);
  };

  if (functionNode.body) {
    visit(functionNode.body);
  }

  return complexity;
}

function collectTopLevelStructureMetrics(sourceFile) {
  let topLevelFunctions = 0;
  let topLevelClasses = 0;
  let topLevelTypeDeclarations = 0;
  const topLevelFunctionNames = [];

  for (const statement of sourceFile.statements) {
    if (ts.isClassDeclaration(statement)) {
      topLevelClasses += 1;
      continue;
    }

    if (
      ts.isInterfaceDeclaration(statement) ||
      ts.isTypeAliasDeclaration(statement) ||
      ts.isEnumDeclaration(statement)
    ) {
      topLevelTypeDeclarations += 1;
      continue;
    }

    if (ts.isFunctionDeclaration(statement)) {
      topLevelFunctions += 1;
      if (statement.name && ts.isIdentifier(statement.name)) {
        topLevelFunctionNames.push(statement.name.text);
      }
      continue;
    }

    if (ts.isVariableStatement(statement)) {
      for (const declaration of statement.declarationList.declarations) {
        const initializer = declaration.initializer;
        if (!initializer) {
          continue;
        }

        if (ts.isArrowFunction(initializer) || ts.isFunctionExpression(initializer)) {
          topLevelFunctions += 1;
          if (ts.isIdentifier(declaration.name)) {
            topLevelFunctionNames.push(declaration.name.text);
          }
        }
      }
    }
  }

  return {
    topLevelFunctions,
    topLevelClasses,
    topLevelTypeDeclarations,
    topLevelFunctionNames
  };
}

const violations = [];
const topLevelFunctionNameUsage = new Map();

for (const file of guardrailFiles) {
  const sourceText = readFileSync(file, "utf8");
  const relativePath = toRepoRelativePath(file);
  const lineCount = sourceText.length === 0 ? 0 : sourceText.split(/\r?\n/).length;

  if (lineCount > MAX_LINES_PER_FILE) {
    violations.push({
      rule: "max-lines",
      file: relativePath,
      line: 1,
      message: `has ${lineCount} lines (max ${MAX_LINES_PER_FILE})`
    });
  }

  const sourceFile = ts.createSourceFile(file, sourceText, ts.ScriptTarget.Latest, true, ts.ScriptKind.TS);
  const structureMetrics = collectTopLevelStructureMetrics(sourceFile);

  if (
    structureMetrics.topLevelFunctions > MAX_TOP_LEVEL_FUNCTIONS_PER_FILE &&
    !LEGACY_MAX_TOP_LEVEL_FUNCTIONS_PER_FILE_EXCEPTIONS.has(relativePath)
  ) {
    violations.push({
      rule: "max-top-level-functions",
      file: relativePath,
      line: 1,
      message: `has ${structureMetrics.topLevelFunctions} top-level functions (max ${MAX_TOP_LEVEL_FUNCTIONS_PER_FILE})`
    });
  }

  if (
    structureMetrics.topLevelClasses > MAX_CLASSES_PER_FILE &&
    !LEGACY_MAX_CLASSES_PER_FILE_EXCEPTIONS.has(relativePath)
  ) {
    violations.push({
      rule: "max-classes-per-file",
      file: relativePath,
      line: 1,
      message: `has ${structureMetrics.topLevelClasses} classes (max ${MAX_CLASSES_PER_FILE})`
    });
  }

  if (
    structureMetrics.topLevelTypeDeclarations > MAX_TYPE_DECLARATIONS_PER_FILE &&
    !LEGACY_MAX_TYPE_DECLARATIONS_PER_FILE_EXCEPTIONS.has(relativePath)
  ) {
    violations.push({
      rule: "max-type-declarations",
      file: relativePath,
      line: 1,
      message: `has ${structureMetrics.topLevelTypeDeclarations} type/interface/enum declarations (max ${MAX_TYPE_DECLARATIONS_PER_FILE})`
    });
  }

  for (const functionName of structureMetrics.topLevelFunctionNames) {
    const filesWithFunction = topLevelFunctionNameUsage.get(functionName) || new Set();
    filesWithFunction.add(relativePath);
    topLevelFunctionNameUsage.set(functionName, filesWithFunction);
  }

  const visit = (node) => {
    if (ts.isFunctionLike(node) && node.body) {
      const startLine = sourceFile.getLineAndCharacterOfPosition(node.getStart(sourceFile)).line + 1;
      const endLine = sourceFile.getLineAndCharacterOfPosition(node.end).line + 1;
      const functionLength = endLine - startLine + 1;
      const name = functionLikeName(node);

      if (functionLength > MAX_LINES_PER_FUNCTION) {
        violations.push({
          rule: "max-lines-per-function",
          file: relativePath,
          line: startLine,
          message: `"${name}" is ${functionLength} lines (max ${MAX_LINES_PER_FUNCTION})`
        });
      }

      const complexity = computeCyclomaticComplexity(node);
      if (complexity > MAX_CYCLOMATIC_COMPLEXITY) {
        violations.push({
          rule: "complexity",
          file: relativePath,
          line: startLine,
          message: `"${name}" has cyclomatic complexity ${complexity} (max ${MAX_CYCLOMATIC_COMPLEXITY})`
        });
      }
    }
    ts.forEachChild(node, visit);
  };
  visit(sourceFile);
}

const commonFunctionCandidates = [];
for (const [functionName, filesWithFunction] of topLevelFunctionNameUsage.entries()) {
  if (filesWithFunction.size < COMMON_FUNCTION_USAGE_THRESHOLD) {
    continue;
  }
  commonFunctionCandidates.push({
    functionName,
    files: [...filesWithFunction].sort()
  });
}
commonFunctionCandidates.sort((a, b) => b.files.length - a.files.length || a.functionName.localeCompare(b.functionName));

if (violations.length > 0) {
  violations.sort((a, b) => {
    const fileCompare = a.file.localeCompare(b.file);
    if (fileCompare !== 0) {
      return fileCompare;
    }
    if (a.line !== b.line) {
      return a.line - b.line;
    }
    return a.rule.localeCompare(b.rule);
  });

  console.error("lint guardrail violations:");
  for (const violation of violations) {
    console.error(`- [${violation.rule}] ${violation.file}:${violation.line} ${violation.message}`);
  }
  process.exit(1);
}

console.log(`checked ${syntaxFiles.length} TypeScript files`);
console.log(
  `guardrails enforced for ${guardrailFiles.length} files ` +
    `(max-lines=${MAX_LINES_PER_FILE}, ` +
    `max-lines-per-function=${MAX_LINES_PER_FUNCTION}, ` +
    `complexity=${MAX_CYCLOMATIC_COMPLEXITY}, ` +
    `max-top-level-functions=${MAX_TOP_LEVEL_FUNCTIONS_PER_FILE}, ` +
    `max-classes-per-file=${MAX_CLASSES_PER_FILE}, ` +
    `max-type-declarations=${MAX_TYPE_DECLARATIONS_PER_FILE})`
);

if (commonFunctionCandidates.length > 0) {
  console.warn("common util candidates (non-blocking):");
  for (const candidate of commonFunctionCandidates.slice(0, 15)) {
    console.warn(
      `- [common-utils] function \"${candidate.functionName}\" appears in ${candidate.files.length} files: ${candidate.files.join(", ")}`
    );
  }
}
